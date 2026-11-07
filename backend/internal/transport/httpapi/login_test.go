package httpapi

import (
	"context"
	"encoding/json"
	"github.com/smofktn/watchbrief/backend/internal/usecase"
	"net/http/httptest"
	"strings"
	"testing"
)

type unavailableLogin struct{}

func (unavailableLogin) Execute(context.Context, string, string) ([32]byte, error) {
	return [32]byte{}, usecase.ErrLogin
}

func TestInputBoundary(t *testing.T) {
	const origin = "http://127.0.0.1:5173"
	const valid = `{"login_id":"Fictional_User","password":"example-only-password"}`
	h, err := NewHandler(origin, unavailableLogin{}, &currentResult{}, &logoutResult{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, body, origin, media, site, code string
		status                                int
	}{
		{"valid input is not authentication", valid, origin, "application/json", "same-site", "internal_error", 500},
		{"NFC maximum decomposed password", `{"login_id":"user","password":"` + strings.Repeat("か\u3099", 128) + `"}`, origin, "application/json", "same-site", "internal_error", 500},
		{"empty body", "", origin, "application/json", "", "invalid_request", 400},
		{"malformed", `{`, origin, "application/json", "", "invalid_request", 400},
		{"missing password", `{"login_id":"user"}`, origin, "application/json", "", "invalid_request", 400},
		{"null password", `{"login_id":"user","password":null}`, origin, "application/json", "", "invalid_request", 400},
		{"wrong type", `{"login_id":42,"password":"example-only-password"}`, origin, "application/json", "", "invalid_request", 400},
		{"short password", `{"login_id":"user","password":"short"}`, origin, "application/json", "", "invalid_request", 400},
		{"duplicate field", `{"login_id":"user","login_id":"other","password":"example-only-password"}`, origin, "application/json", "", "invalid_request", 400},
		{"unknown field", `{"login_id":"user","password":"example-only-password","extra":true}`, origin, "application/json", "", "invalid_request", 400},
		{"trailing object", valid + `{}`, origin, "application/json", "", "invalid_request", 400},
		{"oversized", strings.Repeat(" ", 4097) + valid, origin, "application/json", "", "invalid_request", 400},
		{"wrong media", valid, origin, "text/plain", "", "invalid_request", 400},
		{"missing Origin", valid, "", "application/json", "", "csrf_rejected", 403},
		{"null Origin", valid, "null", "application/json", "", "csrf_rejected", 403},
		{"foreign Origin before parsing", `{`, "http://example.invalid", "application/json", "", "csrf_rejected", 403},
		{"configured trusted origin", valid, origin, "application/json", "cross-site", "internal_error", 500},
		{"foreign origin with same-origin metadata", valid, "http://example.invalid", "application/json", "same-origin", "csrf_rejected", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/login", strings.NewReader(tc.body))
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			r.Header.Set("Content-Type", tc.media)
			if tc.site != "" {
				r.Header.Set("Sec-Fetch-Site", tc.site)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			var got struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal("non-JSON error")
			}
			if w.Code != tc.status || got.Error.Code != tc.code || got.Error.Message == "" {
				t.Fatal("incorrect error classification")
			}
			if w.Header().Get("Set-Cookie") != "" || strings.Contains(w.Body.String(), "example-only-password") {
				t.Fatal("unexpected secret or Cookie")
			}
			if w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing response protection")
			}
		})
	}
}

func TestRejectInvalidOriginConfiguration(t *testing.T) {
	for _, value := range []string{"", "null", "file://localhost", "http://user:password@localhost", "http://localhost/path", "http://localhost?x=1"} {
		if _, err := NewHandler(value, unavailableLogin{}, &currentResult{}, &logoutResult{}, nil); err == nil {
			t.Fatal("accepted invalid origin configuration")
		}
	}
}
