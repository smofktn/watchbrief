package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type LogoutService interface {
	Execute(context.Context, [32]byte) error
}

func logoutHandler(service LogoutService, logger *slog.Logger, secure bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, digest, ok := sessionToken(r)
		if !ok {
			writeError(w, 401, "unauthenticated", "ログインしてください。")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := service.Execute(ctx, digest); err != nil {
			logger.Error("logout_failed", "operation", "logout", "stage", "session_delete", "result", "failed", "version", "issue-54")
			writeError(w, 500, "internal_error", "処理に失敗しました。時間をおいて再度お試しください")
			return
		}
		// Cookie delivery and DB deletion cannot be atomic. A repeated request
		// with the same well-formed token succeeds even if the row is gone.
		http.SetCookie(w, &http.Cookie{Name: SessionCookieName, Value: "", Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0).UTC()})
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	})
}
