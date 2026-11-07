package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/smofktn/watchbrief/backend/internal/usecase"
)

type loginResult struct {
	err   error
	calls int
}

func TestInternalFailureLogsAreSanitized(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	h, err := NewHandler("http://127.0.0.1:5173", &loginResult{err: errors.New("synthetic-private-marker")}, &currentResult{}, &logoutResult{}, logger)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"login_id":"user","password":"example-only-password"}`))
	r.Header.Set("Origin", "http://127.0.0.1:5173")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 500 || !strings.Contains(output.String(), "login_failed") || strings.Contains(output.String(), "synthetic-private-marker") || strings.Contains(output.String(), "example-only-password") || strings.Contains(w.Body.String(), "synthetic-private-marker") {
		t.Fatal("internal response/log sanitization failed")
	}
}

func (s *loginResult) Execute(context.Context, string, string) ([32]byte, error) {
	s.calls++
	var raw [32]byte
	raw[0] = 9
	return raw, s.err
}

func TestCookieIssuedOnlyForSuccess(t *testing.T) {
	for _, origin := range []string{"http://127.0.0.1:5173", "https://watchbrief.example"} {
		for _, failure := range []error{nil, usecase.ErrInvalidCredentials, usecase.ErrLogin} {
			service := &loginResult{err: failure}
			h, err := NewHandler(origin, service, &currentResult{}, &logoutResult{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"login_id":"user","password":"example-only-password"}`))
			r.Header.Set("Origin", origin)
			r.Header.Set("Content-Type", "application/json")
			r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "old-cookie-is-not-authentication"})
			w := httptest.NewRecorder()
			before := time.Now()
			h.ServeHTTP(w, r)
			cookies := w.Result().Cookies()
			if failure != nil {
				want := 500
				if failure == usecase.ErrInvalidCredentials {
					want = 401
				}
				if w.Code != want || len(cookies) != 0 {
					t.Fatal("failure issued a cookie or wrong status")
				}
				continue
			}
			if w.Code != 204 || w.Body.Len() != 0 || len(cookies) != 1 {
				t.Fatal("incorrect successful response")
			}
			c := cookies[0]
			raw, err := base64.RawURLEncoding.Strict().DecodeString(c.Value)
			if err != nil || len(raw) != 32 || raw[0] != 9 {
				t.Fatal("incorrect cookie encoding")
			}
			if c.Name != SessionCookieName || c.Path != "/" || c.Domain != "" || !c.HttpOnly || c.Secure != strings.HasPrefix(origin, "https:") || c.SameSite != http.SameSiteLaxMode || c.MaxAge != 34560000 {
				t.Fatal("incorrect cookie attributes")
			}
			if c.Expires.Before(before.Add(400*24*time.Hour-2*time.Second)) || c.Expires.After(time.Now().Add(400*24*time.Hour+time.Second)) {
				t.Fatal("incorrect browser expiry")
			}
		}
	}
}

func TestOriginCheckedBeforeAuthentication(t *testing.T) {
	service := &loginResult{}
	h, _ := NewHandler("http://127.0.0.1:5173", service, &currentResult{}, &logoutResult{}, nil)
	for _, origins := range [][]string{nil, {"null"}, {"http://other.invalid"}, {"http://127.0.0.1:5173", "http://127.0.0.1:5173"}} {
		r := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{}`))
		for _, v := range origins {
			r.Header.Add("Origin", v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 || service.calls != 0 {
			t.Fatal("invalid Origin reached authentication")
		}
	}
	if _, err := NewHandler("http://watchbrief.example", service, &currentResult{}, &logoutResult{}, nil); err == nil {
		t.Fatal("non-loopback HTTP accepted")
	}
}

func TestUnicodeEscapes(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{`{"login_id":"user","password":"example-password-\ud800"}`, false},
		{`{"login_id":"user","password":"example-password-\udc00"}`, false},
		{`{"login_id":"user","password":"example-password-\ud83d\ude00"}`, true},
		{`{"login_id":"user","password":"literal-password-\\ud800"}`, true},
	} {
		_, _, err := readLogin([]byte(tc.value))
		if (err == nil) != tc.valid {
			t.Fatal("invalid Unicode escape classification")
		}
	}
}
