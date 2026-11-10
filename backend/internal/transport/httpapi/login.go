package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"time"
	"unicode/utf8"

	"github.com/smofktn/watchbrief/backend/internal/domain"
	"github.com/smofktn/watchbrief/backend/internal/usecase"
)

const SessionCookieName = "watchbrief_session"
const SessionCookieMaxAge = 400 * 24 * 60 * 60

type LoginService interface {
	Execute(context.Context, string, string) ([32]byte, error)
}

func NewHandler(origin string, service LoginService, current CurrentUserService, logout LogoutService, logger *slog.Logger, collections ...CollectionService) (http.Handler, error) {
	u, err := url.Parse(origin)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return nil, errors.New("invalid origin configuration")
	}
	if u.Scheme == "http" && u.Hostname() != "localhost" {
		ip := net.ParseIP(u.Hostname())
		if ip == nil || !ip.IsLoopback() {
			return nil, errors.New("HTTP origin must be loopback")
		}
	}
	if service == nil || current == nil || logout == nil {
		return nil, errors.New("authentication services required")
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	protection := http.NewCrossOriginProtection()
	// The trusted origin is still separately required to match exactly.
	if err := protection.AddTrustedOrigin(origin); err != nil {
		return nil, errors.New("invalid origin configuration")
	}
	mux := http.NewServeMux()
	if len(collections) > 0 && collections[0] != nil {
		if creator, ok := collections[0].(CollectionCreator); ok {
			mux.Handle("POST /api/collections", sameOriginHandler(origin, protection, collectionCreationHandler(creator, logger)))
		}
		mux.Handle("GET /api/collections", collectionHandler(collections[0], logger, false))
		mux.Handle("GET /api/collections/{id}", collectionHandler(collections[0], logger, true))
	}
	mux.Handle("GET /api/me", currentUserHandler(current, logger, u.Scheme == "https"))
	mux.Handle("POST /api/logout", sameOriginHandler(origin, protection, logoutHandler(logout, logger, u.Scheme == "https")))
	mux.Handle("POST /api/login", sameOriginHandler(origin, protection, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeError(w, 400, "invalid_request", "JSON形式でログインIDとパスワードを指定してください。")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil || !utf8.Valid(body) {
			writeError(w, 400, "invalid_request", "入力内容を確認してください。")
			return
		}
		id, password, err := readLogin(body)
		if err == nil {
			_, err = domain.ValidateLoginInput(id, password)
		}
		if err != nil {
			writeError(w, 400, "invalid_request", "ログインIDは英数字・_・-の1〜64文字、パスワードは15〜128文字で入力してください。")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		raw, err := service.Execute(ctx, id, password)
		if err != nil {
			switch {
			case errors.Is(err, usecase.ErrInvalidCredentials):
				writeError(w, 401, "invalid_credentials", "ログインIDまたはパスワードが違います")
			case errors.Is(err, domain.ErrInvalidCredentialsInput):
				writeError(w, 400, "invalid_request", "入力内容を確認してください。")
			default:
				logger.Error("login_failed", "operation", "login", "stage", "authenticate_or_save", "result", "failed", "version", "issue-53")
				writeError(w, 500, "internal_error", "処理に失敗しました。時間をおいて再度お試しください")
			}
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		setSessionCookie(w, base64.RawURLEncoding.EncodeToString(raw[:]), u.Scheme == "https")
		w.WriteHeader(http.StatusNoContent)
	})))
	return mux, nil
}

func readLogin(body []byte) (string, string, error) {
	if !validJSONUnicode(body) {
		return "", "", errors.New("invalid Unicode escape")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return "", "", errors.New("invalid JSON object")
	}
	fields := make(map[string]string, 2)
	for d.More() {
		t, err = d.Token()
		if err != nil {
			return "", "", errors.New("invalid JSON field")
		}
		name, ok := t.(string)
		_, duplicate := fields[name]
		if !ok || (name != "login_id" && name != "password") || duplicate {
			return "", "", errors.New("invalid JSON field")
		}
		var value *string
		if err := d.Decode(&value); err != nil || value == nil {
			return "", "", errors.New("invalid JSON value")
		}
		fields[name] = *value
	}
	if _, err := d.Token(); err != nil {
		return "", "", errors.New("invalid JSON object")
	}
	if d.Decode(new(any)) != io.EOF || len(fields) != 2 {
		return "", "", errors.New("invalid JSON body")
	}
	return fields["login_id"], fields["password"], nil
}

// encoding/json replaces unpaired UTF-16 surrogates. Authentication rejects them
// rather than silently treating them as the replacement character in a password.
func validJSONUnicode(body []byte) bool {
	hex4 := func(value []byte) (uint16, bool) {
		if len(value) != 4 {
			return 0, false
		}
		var result uint16
		for _, b := range value {
			result *= 16
			switch {
			case b >= '0' && b <= '9':
				result += uint16(b - '0')
			case b >= 'a' && b <= 'f':
				result += uint16(b - 'a' + 10)
			case b >= 'A' && b <= 'F':
				result += uint16(b - 'A' + 10)
			default:
				return 0, false
			}
		}
		return result, true
	}
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' {
			continue
		}
		i++
		if i >= len(body) {
			return false
		}
		if body[i] != 'u' {
			continue
		}
		if i+4 >= len(body) {
			return false
		}
		value, ok := hex4(body[i+1 : i+5])
		if !ok {
			return false
		}
		i += 4
		if value >= 0xDC00 && value <= 0xDFFF {
			return false
		}
		if value >= 0xD800 && value <= 0xDBFF {
			if i+6 >= len(body) || body[i+1] != '\\' || body[i+2] != 'u' {
				return false
			}
			low, ok := hex4(body[i+3 : i+7])
			if !ok || low < 0xDC00 || low > 0xDFFF {
				return false
			}
			i += 6
		}
	}
	return true
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// Only fixed, non-secret strings are serialized. Encoding cannot fail for this value.
	_ = json.NewEncoder(w).Encode(struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}{Error: struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{code, message}})
}
