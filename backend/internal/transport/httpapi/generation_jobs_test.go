package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type jobResult struct {
	calls int
	err   error
}

func (s *jobResult) List(context.Context, [32]byte, string) (domain.GenerationJobPage, error) {
	s.calls++
	return domain.GenerationJobPage{GenerationJobs: []domain.GenerationJobSummary{}}, s.err
}
func TestGenerationJobHTTPBoundary(t *testing.T) {
	valid := SessionCookieName + "=" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	for _, tc := range []struct {
		name, cookie, path string
		err                error
		status             int
		code               string
	}{
		{"empty list", valid, "/api/generation-jobs", nil, 200, "generation_jobs"},
		{"no cookie", "", "/api/generation-jobs", nil, 401, "unauthenticated"},
		{"duplicate cookie", valid + "; " + valid, "/api/generation-jobs", nil, 401, "unauthenticated"},
		{"invalid cookie", SessionCookieName + "=invalid", "/api/generation-jobs", nil, 401, "unauthenticated"},
		{"unknown session", valid, "/api/generation-jobs", domain.ErrUnauthenticated, 401, "unauthenticated"},
		{"unknown session invalid query", valid, "/api/generation-jobs?cursor=%xx", domain.ErrUnauthenticated, 401, "unauthenticated"},
		{"invalid cursor", valid, "/api/generation-jobs?cursor=private-cursor", domain.ErrGenerationJobCursor, 400, "invalid_request"},
		{"DB failure", valid, "/api/generation-jobs", errors.New("private SQL connection data"), 500, "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			service := &jobResult{err: tc.err}
			h, err := NewHandler("http://127.0.0.1:5173", &loginResult{}, &currentResult{}, &logoutResult{}, slog.New(slog.NewJSONHandler(&logs, nil)))
			if err != nil {
				t.Fatal(err)
			}
			h = WithGenerationJobs(h, service, slog.New(slog.NewJSONHandler(&logs, nil)))
			r := httptest.NewRequest("GET", tc.path, nil)
			r.Header.Set("Cookie", tc.cookie)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("wrong resource classification")
			}
			if strings.Contains(w.Body.String(), "private SQL") || strings.Contains(logs.String(), "private SQL") || strings.Contains(logs.String(), valid) || strings.Contains(logs.String(), "private-cursor") {
				t.Fatal("private value exposed")
			}
			if tc.name == "no cookie" || tc.name == "duplicate cookie" || tc.name == "invalid cookie" {
				if service.calls != 0 {
					t.Fatal("invalid Cookie reached service")
				}
			}
			if tc.name == "empty list" && (!strings.Contains(w.Body.String(), `"generation_jobs":[]`) || !strings.Contains(w.Body.String(), `"next_cursor":null`)) {
				t.Fatal("empty result must be array")
			}
		})
	}
}
