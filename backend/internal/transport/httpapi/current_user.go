package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/smofktn/watchbrief/backend/internal/domain"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type CurrentUserService interface {
	Execute(context.Context, [32]byte) (string, error)
}

// Inspect matching names before net/http drops malformed cookie values.
// Duplicate, quoted, padded and noncanonical authentication tokens are rejected.
func sessionToken(r *http.Request) (string, [32]byte, bool) {
	var token string
	count := 0
	for _, line := range r.Header.Values("Cookie") {
		for _, part := range strings.Split(line, ";") {
			name, value, hasValue := strings.Cut(strings.TrimSpace(part), "=")
			if name != SessionCookieName {
				continue
			}
			count++
			if !hasValue {
				return "", [32]byte{}, false
			}
			token = value
		}
	}
	if count != 1 || len(token) != 43 {
		return "", [32]byte{}, false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return "", [32]byte{}, false
	}
	return token, sha256.Sum256(raw), true
}

func setSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: token, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: SessionCookieMaxAge, Expires: time.Now().UTC().Add(time.Duration(SessionCookieMaxAge) * time.Second)})
}

func currentUserHandler(service CurrentUserService, logger *slog.Logger, secure bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value, digest, ok := sessionToken(r)
		if !ok {
			writeError(w, 401, "unauthenticated", "ログインしてください。")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		loginID, err := service.Execute(ctx, digest)
		if err != nil {
			if errors.Is(err, domain.ErrUnauthenticated) {
				writeError(w, 401, "unauthenticated", "ログインしてください。")
			} else {
				logger.Error("current_user_failed", "operation", "current_user", "stage", "session_lookup", "result", "failed", "version", "issue-53")
				writeError(w, 500, "internal_error", "処理に失敗しました。時間をおいて再度お試しください")
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		setSessionCookie(w, value, secure)
		_ = json.NewEncoder(w).Encode(struct {
			LoginID string `json:"login_id"`
		}{loginID})
	})
}
