package httpapi

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "io"
 "log/slog"
 "mime"
 "net/http"
 "strings"
 "time"
 "unicode/utf8"

 "github.com/smofktn/watchbrief/backend/internal/auth"
 "github.com/smofktn/watchbrief/backend/internal/store"
)

type Repository interface {
 UserByLogin(context.Context, string) (store.User, error)
 UserByToken(context.Context, []byte) (store.User, error)
 CreateSession(context.Context, string, string, []byte) error
}

type API struct {
 repo Repository
 log *slog.Logger
 origin, cookieName, dummyHash string
 secure bool
 hashSlots chan struct{}
}

func New(repo Repository, log *slog.Logger, origin string, secure bool) (*API, error) {
 dummy, err := auth.NewToken()
 if err != nil { return nil, err }
 dummyHash, err := auth.HashPassword(dummy)
 if err != nil { return nil, err }
 name := "__Host-watchbrief_session"
 if !secure { name = "watchbrief_session_local" }
 return &API{repo:repo, log:log, origin:origin, secure:secure, cookieName:name, dummyHash:dummyHash, hashSlots:make(chan struct{}, 2)}, nil
}

func (a *API) Handler() http.Handler {
 mux := http.NewServeMux()
 mux.HandleFunc("POST /api/login", a.login)
 mux.HandleFunc("GET /api/me", a.me)
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Cache-Control", "no-store")
  w.Header().Set("X-Content-Type-Options", "nosniff")
  requestID, err := auth.NewID()
  if err != nil { a.log.Error("request_failed", "operation", "request", "stage", "random", "result", "internal_error"); failure(w, 500, "internal_error"); return }
  w.Header().Set("X-Request-ID", requestID)
  // Log only fixed categories; never raw errors, URLs, request bodies or headers.
  defer func() {
   if recover() != nil { a.internal(w, requestID, "request", "panic") }
  }()
  ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
  defer cancel()
  r = r.WithContext(context.WithValue(ctx, requestIDKey{}, requestID))
  // All future state changes must go through this same boundary, before authentication.
  if r.Method != http.MethodGet && r.Method != http.MethodHead {
   if r.Header.Get("Origin") != a.origin || r.Header.Get("X-WatchBrief-CSRF") != "1" || (r.Header.Get("Sec-Fetch-Site") != "" && r.Header.Get("Sec-Fetch-Site") != "same-origin") {
    failure(w, http.StatusForbidden, "csrf_rejected"); return
   }
   mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
   if err != nil || mediaType != "application/json" { failure(w, http.StatusUnsupportedMediaType, "json_required"); return }
  }
  // No CORS allow headers; cross-origin JS cannot complete a preflight.
  mux.ServeHTTP(w, r)
 })
}

type requestIDKey struct{}
func (a *API) internal(w http.ResponseWriter, requestID, operation, stage string) {
 a.log.Error("request_failed", "request_id", requestID, "operation", operation, "stage", stage, "result", "internal_error", "version", "issue-53")
 failure(w, 500, "internal_error")
}
func failure(w http.ResponseWriter, status int, code string) { respond(w, status, map[string]string{"error":code}) }
func respond(w http.ResponseWriter, status int, value any) {
 w.Header().Set("Content-Type", "application/json; charset=utf-8")
 w.WriteHeader(status)
 _ = json.NewEncoder(w).Encode(value) // Response contains only explicit public DTOs.
}

const cookieMaxAge = 400 * 24 * 60 * 60
func (a *API) cookie(w http.ResponseWriter, token string) {
 http.SetCookie(w, &http.Cookie{Name:a.cookieName, Value:token, Path:"/", HttpOnly:true, Secure:a.secure, SameSite:http.SameSiteLaxMode, MaxAge:cookieMaxAge})
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
 requestID, _ := r.Context().Value(requestIDKey{}).(string)
 var input struct { LoginID string `json:"login_id"`; Password string `json:"password"` }
 body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
 if err != nil || !utf8.Valid(body) { failure(w, 400, "invalid_input"); return }
 dec := json.NewDecoder(bytes.NewReader(body))
 dec.DisallowUnknownFields()
 if err := dec.Decode(&input); err != nil { failure(w, 400, "invalid_input"); return }
 var extra any
 if dec.Decode(&extra) != io.EOF { failure(w, 400, "invalid_input"); return }
 login, valid := auth.LoginID(input.LoginID)
 if !valid || !auth.ValidPassword(input.Password) { failure(w, 400, "invalid_input"); return }
 // Bound Argon2 memory concurrency without introducing account/attempt throttling (#49).
 select {
 case a.hashSlots <- struct{}{}:
  defer func(){ <-a.hashSlots }()
 case <-r.Context().Done():
  a.internal(w, requestID, "login", "hash_capacity"); return
 }
 user, err := a.repo.UserByLogin(r.Context(), login)
 missing := errors.Is(err, store.ErrNotFound)
 if err != nil && !missing { a.internal(w, requestID, "login", "user_lookup"); return }
 encoded := user.PasswordHash
 if missing { encoded = a.dummyHash }
 valid, err = auth.VerifyPassword(input.Password, encoded)
 input.Password = ""
 if err != nil { a.internal(w, requestID, "login", "password_hash"); return }
 if missing || !valid { failure(w, 401, "invalid_credentials"); return }
 token, err := auth.NewToken()
 if err != nil { a.internal(w, requestID, "login", "token_random"); return }
 id, err := auth.NewID()
 if err != nil { a.internal(w, requestID, "login", "id_random"); return }
 digest, _ := auth.TokenHash(token)
 if err := a.repo.CreateSession(r.Context(), id, user.ID, digest); err != nil { a.internal(w, requestID, "login", "session_save"); return }
 // Always a new token; never promote an incoming cookie to an authenticated session.
 a.cookie(w, token)
 respond(w, 200, map[string]string{"result":"ok"})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
 requestID, _ := r.Context().Value(requestIDKey{}).(string)
 // Reject duplicates to avoid ambiguous Path/Domain cookie shadowing.
 var token string
 count := 0
 for _, c := range r.Cookies() { if c.Name == a.cookieName { token = c.Value; count++ } }
 if count != 1 { failure(w, 401, "unauthenticated"); return }
 digest, valid := auth.TokenHash(token)
 if !valid || strings.TrimSpace(token) != token { failure(w, 401, "unauthenticated"); return }
 user, err := a.repo.UserByToken(r.Context(), digest)
 if errors.Is(err, store.ErrNotFound) { failure(w, 401, "unauthenticated"); return }
 if err != nil { a.internal(w, requestID, "me", "session_lookup"); return }
 a.cookie(w, token) // Refresh browser retention only; DB authentication has no clock expiry.
 respond(w, 200, struct { ID string `json:"id"`; LoginID string `json:"login_id"` }{user.ID, user.LoginID})
}
