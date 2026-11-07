package httpapi

import "net/http"

func sameOriginHandler(origin string, protection *http.CrossOriginProtection, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header.Values("Origin")) != 1 || r.Header.Get("Origin") != origin || protection.Check(r) != nil {
			writeError(w, 403, "csrf_rejected", "リクエストの送信元を確認できません。")
			return
		}
		next.ServeHTTP(w, r)
	})
}
