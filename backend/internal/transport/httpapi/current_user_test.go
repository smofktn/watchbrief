package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/smofktn/watchbrief/backend/internal/domain"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type currentResult struct {
	calls  int
	digest [32]byte
	err    error
}

func (s *currentResult) Execute(_ context.Context, digest [32]byte) (string, error) {
	s.calls++
	s.digest = digest
	return "database_user", s.err
}

func TestCurrentUserRejectsInvalidCookiesBeforeDB(t *testing.T) {
	raw := make([]byte, 32)
	valid := base64.RawURLEncoding.EncodeToString(raw)
	for _, header := range []string{"", "other=value", SessionCookieName, SessionCookieName + "=", SessionCookieName + "=bad", SessionCookieName + "=" + valid + "=", SessionCookieName + "=\"" + valid + "\"", SessionCookieName + "=" + valid[:42] + "B", SessionCookieName + "=" + valid + "; " + SessionCookieName + "=bad", SessionCookieName + "=" + strings.Repeat("A", 1000)} {
		service := &currentResult{}
		h, err := NewHandler("http://127.0.0.1:5173", &loginResult{}, service, nil)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("GET", "/api/me", nil)
		r.Header.Set("Cookie", header)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 401 || service.calls != 0 || len(w.Result().Cookies()) != 0 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("invalid Cookie reached DB or changed Cookie")
		}
	}
	// Duplicate names split across header lines, including a malformed value.
	service := &currentResult{}
	h, _ := NewHandler("http://127.0.0.1:5173", &loginResult{}, service, nil)
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.Header.Add("Cookie", SessionCookieName+"="+valid)
	r.Header.Add("Cookie", SessionCookieName+"=\"bad value\"")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 || service.calls != 0 {
		t.Fatal("duplicate header accepted")
	}
}

func TestCurrentUserResponseAndRenewal(t *testing.T) {
	raw := [32]byte{5}
	value := base64.RawURLEncoding.EncodeToString(raw[:])
	for _, origin := range []string{"http://127.0.0.1:5173", "https://watchbrief.example"} {
		for _, failure := range []error{nil, domain.ErrUnauthenticated, errors.New("private-db-token-marker")} {
			service := &currentResult{err: failure}
			var logs bytes.Buffer
			h, err := NewHandler(origin, &loginResult{}, service, slog.New(slog.NewJSONHandler(&logs, nil)))
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("GET", "/api/me?login_id=someone_else", nil)
			r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: value})
			w := httptest.NewRecorder()
			before := time.Now()
			h.ServeHTTP(w, r)
			if service.calls != 1 || service.digest != sha256.Sum256(raw[:]) {
				t.Fatal("wrong token digest")
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("cache enabled")
			}
			cookies := w.Result().Cookies()
			if failure != nil {
				want := 500
				if errors.Is(failure, domain.ErrUnauthenticated) {
					want = 401
				}
				if w.Code != want || len(cookies) != 0 || strings.Contains(w.Body.String(), "private-db-token-marker") || strings.Contains(logs.String(), "private-db-token-marker") || strings.Contains(logs.String(), value) {
					t.Fatal("unsafe failure response")
				}
				continue
			}
			var body map[string]string
			if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil || len(body) != 1 || body["login_id"] != "database_user" {
				t.Fatal("incorrect identity response")
			}
			if len(cookies) != 1 {
				t.Fatal("no renewal")
			}
			c := cookies[0]
			if c.Value != value || c.Path != "/" || c.Domain != "" || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Secure != strings.HasPrefix(origin, "https:") || c.MaxAge != 34560000 || c.Expires.Before(before.Add(400*24*time.Hour-time.Second)) {
				t.Fatal("incorrect renewal")
			}
		}
	}
}
