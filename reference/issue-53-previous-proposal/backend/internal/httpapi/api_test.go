package httpapi

import (
 "bytes"
 "context"
 "encoding/hex"
 "encoding/json"
 "errors"
 "log/slog"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"

 "github.com/smofktn/watchbrief/backend/internal/auth"
 "github.com/smofktn/watchbrief/backend/internal/store"
)

type memoryRepo struct { user store.User; tokens map[string]bool; failAt string }
func (m *memoryRepo) UserByLogin(_ context.Context, login string) (store.User, error) {
 if m.failAt == "panic" { panic("NEVER_PRINT_THIS") }
 if m.failAt == "lookup" { return store.User{}, errors.New("driver secret: NEVER_PRINT_THIS") }
 if login != m.user.LoginID { return store.User{}, store.ErrNotFound }
 return m.user, nil
}
func (m *memoryRepo) UserByToken(_ context.Context, hash []byte) (store.User, error) {
 if m.failAt == "me" { return store.User{}, errors.New("driver secret: NEVER_PRINT_THIS") }
 if !m.tokens[hex.EncodeToString(hash)] { return store.User{}, store.ErrNotFound }
 return m.user, nil
}
func (m *memoryRepo) CreateSession(_ context.Context, _, _ string, hash []byte) error {
 if m.failAt == "save" { return errors.New("driver secret: NEVER_PRINT_THIS") }
 m.tokens[hex.EncodeToString(hash)] = true
 return nil
}
func setup(t *testing.T, secure bool) (*API, *memoryRepo, *bytes.Buffer) {
 t.Helper()
 hash, err := auth.HashPassword("Fictional password for tests!")
 if err != nil { t.Fatal(err) }
 repo := &memoryRepo{user:store.User{ID:"01ARZ3NDEKTSV4RRFFQ69G5FAV", LoginID:"fictional",PasswordHash:hash}, tokens:map[string]bool{}}
 var logs bytes.Buffer
 api, err := New(repo,slog.New(slog.NewJSONHandler(&logs,nil)),"http://127.0.0.1:5173",secure)
 if err != nil { t.Fatal(err) }
 return api,repo,&logs
}
func call(api *API, method, path, body string, headers map[string]string, cookie *http.Cookie) *httptest.ResponseRecorder {
 req := httptest.NewRequest(method,path,strings.NewReader(body))
 for k,v := range headers { req.Header.Set(k,v) }
 if cookie != nil { req.AddCookie(cookie) }
 res := httptest.NewRecorder()
 api.Handler().ServeHTTP(res,req)
 return res
}
var headers = map[string]string{"Origin":"http://127.0.0.1:5173", "Content-Type":"application/json", "X-WatchBrief-CSRF":"1"}
const loginBody = `{"login_id":"Fictional","password":"Fictional password for tests!"}`

func TestLoginMeAndNewAPIInstance(t *testing.T) {
 api,repo,logs := setup(t,false)
 res := call(api,"GET","/api/me","",nil,nil)
 if res.Code != 401 { t.Fatal("anonymous accepted") }
 res = call(api,"POST","/api/login",loginBody,headers,nil)
 if res.Code != 200 { t.Fatalf("login status: %d",res.Code) }
 cookies := res.Result().Cookies()
 if len(cookies) != 1 { t.Fatal("missing cookie") }
 cookie := cookies[0]
 if !cookie.HttpOnly || cookie.Secure || cookie.Domain != "" || cookie.Path != "/" || cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != cookieMaxAge { t.Fatal("wrong cookie policy") }
 digest, valid := auth.TokenHash(cookie.Value)
 if !valid || !repo.tokens[hex.EncodeToString(digest)] { t.Fatal("token hash was not persisted") }
 second,err := New(repo,slog.New(slog.NewJSONHandler(logs,nil)),api.origin,false)
 if err != nil { t.Fatal(err) }
 res = call(second,"GET","/api/me","",nil,cookie)
 if res.Code != 200 || !strings.Contains(res.Body.String(),`"login_id":"fictional"`) { t.Fatal("identity failed with new API instance") }
 if len(res.Result().Cookies()) != 1 || res.Result().Cookies()[0].Value != cookie.Value || res.Header().Get("Cache-Control") != "no-store" { t.Fatal("retention was not refreshed") }
 if strings.Contains(res.Body.String(),"password") || strings.Contains(res.Body.String(),cookie.Value) { t.Fatal("secret in public identity DTO") }
 res2 := call(second,"POST","/api/login",loginBody,headers,cookie)
 if res2.Code != 200 || res2.Result().Cookies()[0].Value == cookie.Value { t.Fatal("login reused token") }
 // This test checks new instance construction with a shared repository, not real process/DB restart.
 if logs.Len() != 0 { t.Fatal("unexpected logs") }
}

func TestInputCredentialsAndCSRF(t *testing.T) {
 api,repo,_ := setup(t,false)
 cases := []struct { name,body string; change map[string]string; status int }{
  {"wrong password",strings.Replace(loginBody,"tests!","wrong!",1),nil,401},
  {"missing user",strings.Replace(loginBody,"Fictional\"","absent\"",1),nil,401},
  {"bad id",strings.Replace(loginBody,"Fictional\"","bad@id\"",1),nil,400},
  {"short password",`{"login_id":"fictional","password":"short"}`,nil,400},
  {"invalid JSON","{",nil,400},
  {"invalid UTF-8",strings.Replace(loginBody,"tests!",string([]byte{255}),1),nil,400},
  {"unknown field",strings.TrimSuffix(loginBody,"}")+`,"extra":1}`,nil,400},
  {"trailing JSON",loginBody+` {}`,nil,400},
  {"too large",strings.Repeat("x",5000),nil,400},
  {"no origin",loginBody,map[string]string{"Origin":""},403},
  {"null origin",loginBody,map[string]string{"Origin":"null"},403},
  {"evil origin",loginBody,map[string]string{"Origin":"https://evil.invalid"},403},
  {"sibling origin",loginBody,map[string]string{"Origin":"http://localhost:5173"},403},
  {"origin suffix",loginBody,map[string]string{"Origin":"http://127.0.0.1:5173.evil.invalid"},403},
  {"no header",loginBody,map[string]string{"X-WatchBrief-CSRF":""},403},
  {"cross-site",loginBody,map[string]string{"Sec-Fetch-Site":"cross-site"},403},
  {"same-site",loginBody,map[string]string{"Sec-Fetch-Site":"same-site"},403},
  {"form",loginBody,map[string]string{"Content-Type":"application/x-www-form-urlencoded"},415},
  {"plain",loginBody,map[string]string{"Content-Type":"text/plain"},415},
 }
 for _,tc := range cases { t.Run(tc.name,func(t *testing.T){
  hs := map[string]string{}
  for k,v := range headers { hs[k]=v }
  for k,v := range tc.change { hs[k]=v }
  res := call(api,"POST","/api/login",tc.body,hs,nil)
  if res.Code != tc.status || len(res.Result().Cookies()) != 0 { t.Fatalf("status %d want %d, or unexpected cookie",res.Code,tc.status) }
 }) }
 if len(repo.tokens) != 0 { t.Fatal("failed request created a session") }
 // Browser preflight receives no CORS permission.
 res := call(api,"OPTIONS","/api/login","",map[string]string{"Origin":"https://evil.invalid","Access-Control-Request-Method":"POST"},nil)
 if res.Header().Get("Access-Control-Allow-Origin") != "" { t.Fatal("CORS accidentally allowed") }
}

func TestInternalFailuresAreSanitized(t *testing.T) {
 api,repo,logs := setup(t,false)
 for _,stage := range []string{"lookup","save","panic"} {
  repo.failAt=stage
  res := call(api,"POST","/api/login",loginBody,headers,nil)
  if res.Code != 500 || len(res.Result().Cookies()) != 0 || strings.Contains(res.Body.String(),"NEVER_PRINT") { t.Fatal("unsafe internal failure") }
 }
 repo.failAt=""
 cookie := call(api,"POST","/api/login",loginBody,headers,nil).Result().Cookies()[0]
 repo.failAt="me"
 res := call(api,"GET","/api/me","",nil,cookie)
 if res.Code != 500 || len(res.Result().Cookies()) != 0 { t.Fatal("DB failure should not renew cookie") }
 for _,secret := range []string{"NEVER_PRINT_THIS", "Fictional password", cookie.Value, repo.user.PasswordHash, "fictional"} {
  if strings.Contains(logs.String(),secret) { t.Fatal("secret or input in error logs") }
 }
 for _,line := range strings.Split(strings.TrimSpace(logs.String()),"\n") {
  var row map[string]any
  if json.Unmarshal([]byte(line),&row) != nil || row["request_id"] == nil || row["stage"] == nil { t.Fatal("unstructured error") }
 }
}

func TestSecureCookieAndMalformedCookies(t *testing.T) {
 api,_,_ := setup(t,true)
 res := call(api,"POST","/api/login",loginBody,headers,nil)
 cookie := res.Result().Cookies()[0]
 if cookie.Name != "__Host-watchbrief_session" || !cookie.Secure || !cookie.HttpOnly || cookie.Domain != "" { t.Fatal("HTTPS cookie policy") }
 for _,value := range []string{"junk",strings.Repeat("a",64)} {
  res := call(api,"GET","/api/me","",nil,&http.Cookie{Name:cookie.Name,Value:value})
  if res.Code != 401 { t.Fatal("invalid token accepted") }
 }
 req := httptest.NewRequest("GET","/api/me",nil)
 req.AddCookie(cookie); req.AddCookie(cookie)
 rec := httptest.NewRecorder(); api.Handler().ServeHTTP(rec,req)
 if rec.Code != 401 { t.Fatal("duplicate cookies accepted") }
}
