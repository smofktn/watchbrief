package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type logoutResult struct {
	calls  int
	digest [32]byte
	err    error
}

func (s *logoutResult) Execute(_ context.Context, digest [32]byte) error {
	s.calls++
	s.digest = digest
	return s.err
}

func TestLogoutRejectsBeforeDeleting(t *testing.T) {
	const origin = "http://127.0.0.1:5173"
	value := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	valid := SessionCookieName + "=" + value
	for _, tc := range []struct {
		name, method     string
		origins, cookies []string
		status           int
		code             string
	}{
		{"GET", "GET", []string{origin}, []string{valid}, 405, ""},
		{"HEAD", "HEAD", []string{origin}, []string{valid}, 405, ""},
		{"missing origin", "POST", nil, []string{valid}, 403, "csrf_rejected"},
		{"null origin", "POST", []string{"null"}, []string{valid}, 403, "csrf_rejected"},
		{"foreign origin", "POST", []string{"https://external.invalid"}, []string{valid}, 403, "csrf_rejected"},
		{"different port", "POST", []string{"http://127.0.0.1:5174"}, []string{valid}, 403, "csrf_rejected"},
		{"duplicate origin", "POST", []string{origin, origin}, []string{valid}, 403, "csrf_rejected"},
		{"missing cookie", "POST", []string{origin}, nil, 401, "unauthenticated"},
		{"invalid cookie", "POST", []string{origin}, []string{SessionCookieName + "=bad"}, 401, "unauthenticated"},
		{"noncanonical cookie", "POST", []string{origin}, []string{SessionCookieName + "=" + value[:42] + "B"}, 401, "unauthenticated"},
		{"quoted cookie", "POST", []string{origin}, []string{SessionCookieName + "=\"" + value + "\""}, 401, "unauthenticated"},
		{"padded cookie", "POST", []string{origin}, []string{valid + "="}, 401, "unauthenticated"},
		{"duplicate cookie", "POST", []string{origin}, []string{valid + "; " + SessionCookieName + "=bad"}, 401, "unauthenticated"},
		{"duplicate header malformed value", "POST", []string{origin}, []string{valid, SessionCookieName + "=\"bad value\""}, 401, "unauthenticated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &logoutResult{}
			h, err := NewHandler(origin, &loginResult{}, &currentResult{}, service, nil)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(tc.method, "/api/logout", nil)
			for _, v := range tc.origins {
				r.Header.Add("Origin", v)
			}
			for _, v := range tc.cookies {
				r.Header.Add("Cookie", v)
			}
			// Untrusted Origin must fail even if browser metadata says same-origin.
			r.Header.Set("Sec-Fetch-Site", "same-origin")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || service.calls != 0 || w.Header().Get("Set-Cookie") != "" {
				t.Fatal("rejected request reached deletion or changed Cookie")
			}
			if tc.code != "" {
				var body struct{ Error struct{ Code string } }
				if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Error.Code != tc.code || w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("incorrect failure classification")
				}
			}
		})
	}
}

func TestLogoutDeletesThenClearsCookieAndAllowsRetry(t *testing.T) {
	raw := [32]byte{54}
	value := base64.RawURLEncoding.EncodeToString(raw[:])
	for _, origin := range []string{"http://127.0.0.1:5173", "https://watchbrief.example"} {
		service := &logoutResult{err: errors.New("private-driver-detail")}
		var logs bytes.Buffer
		h, err := NewHandler(origin, &loginResult{}, &currentResult{}, service, slog.New(slog.NewJSONHandler(&logs, nil)))
		if err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 3; attempt++ {
			r := httptest.NewRequest("POST", "/api/logout", nil)
			r.Header.Set("Origin", origin)
			r.Header.Set("Sec-Fetch-Site", "same-origin")
			r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: value})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if service.calls != attempt+1 || service.digest != sha256.Sum256(raw[:]) {
				t.Fatal("incorrect session selected")
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("cache enabled")
			}
			if attempt == 0 {
				if w.Code != 500 || len(w.Result().Cookies()) != 0 || !strings.Contains(w.Body.String(), "internal_error") {
					t.Fatal("DB failure reported as success or removed Cookie")
				}
				if !strings.Contains(logs.String(), "session_delete") {
					t.Fatal("missing failure stage")
				}
				for _, secret := range []string{"private-driver-detail", value} {
					if strings.Contains(w.Body.String(), secret) || strings.Contains(logs.String(), secret) {
						t.Fatal("secret exposed")
					}
				}
				service.err = nil
				continue
			}
			cookies := w.Result().Cookies()
			if w.Code != 204 || w.Body.Len() != 0 || len(cookies) != 1 {
				t.Fatal("retry failed")
			}
			c := cookies[0]
			if c.Name != SessionCookieName || c.Value != "" || c.MaxAge != -1 || c.Expires.IsZero() || !c.Expires.Before(time.Now()) || c.Path != "/" || c.Domain != "" || !c.HttpOnly || c.Secure != strings.HasPrefix(origin, "https:") || c.SameSite != http.SameSiteLaxMode {
				t.Fatal("incorrect Cookie deletion attributes")
			}
		}
	}
}
