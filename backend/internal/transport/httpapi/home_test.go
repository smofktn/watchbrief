package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type homeResultStub struct {
	calls  int
	digest [32]byte
	query  string
	value  domain.Home
	err    error
}

func (s *homeResultStub) List(_ context.Context, digest [32]byte, query string) (domain.Home, error) {
	s.calls++
	s.digest = digest
	s.query = query
	return s.value, s.err
}

func TestHomeHTTPBoundaryAndMinimalResponse(t *testing.T) {
	raw := [32]byte{7}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	validCookie := SessionCookieName + "=" + token
	reportID := "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	for _, tc := range []struct {
		name, cookie, path string
		err                error
		status             int
		contains           string
	}{
		{name: "success", cookie: validCookie, path: "/api/home", status: 200, contains: `"latest_report_id":"` + reportID + `"`},
		{name: "no cookie", path: "/api/home", status: 401, contains: "unauthenticated"},
		{name: "duplicate cookie", cookie: validCookie + "; " + validCookie, path: "/api/home", status: 401, contains: "unauthenticated"},
		{name: "invalid cookie", cookie: SessionCookieName + "=invalid", path: "/api/home", status: 401, contains: "unauthenticated"},
		{name: "expired session", cookie: validCookie, path: "/api/home", err: domain.ErrUnauthenticated, status: 401, contains: "unauthenticated"},
		{name: "invalid query", cookie: validCookie, path: "/api/home?cursor=opaque", err: domain.ErrHomeRequest, status: 400, contains: "invalid_request"},
		{name: "database failure", cookie: validCookie, path: "/api/home", err: errors.New("private SQL connection marker"), status: 500, contains: "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			service := &homeResultStub{
				value: domain.Home{Collections: []domain.HomeCollection{
					{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Name: "current collection", LatestReportID: &reportID},
					{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAX", Name: "no report"},
				}},
				err: tc.err,
			}
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			h, err := NewHandler("http://127.0.0.1:5173", &loginResult{}, &currentResult{}, &logoutResult{}, logger)
			if err != nil {
				t.Fatal(err)
			}
			h = WithHome(h, service, logger)
			r := httptest.NewRequest("GET", tc.path, nil)
			r.Header.Set("Cookie", tc.cookie)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("home response classification or caching is wrong")
			}
			if tc.name == "success" {
				if service.calls != 1 || service.query != "" || service.digest != sha256.Sum256(raw[:]) || len(w.Result().Cookies()) != 0 {
					t.Fatal("home request did not use the session digest read-only")
				}
				for _, forbidden := range []string{"summary_markdown", "body_markdown", "input_snapshot", "collection_name", "private"} {
					if strings.Contains(w.Body.String(), forbidden) {
						t.Fatal("home response exposed non-summary data")
					}
				}
			}
			if tc.name == "no cookie" || tc.name == "duplicate cookie" || tc.name == "invalid cookie" {
				if service.calls != 0 {
					t.Fatal("invalid Cookie reached the service")
				}
			}
			if strings.Contains(w.Body.String(), "private SQL connection marker") || strings.Contains(logs.String(), "private SQL connection marker") || strings.Contains(logs.String(), token) {
				t.Fatal("private values leaked to response or logs")
			}
		})
	}
}
