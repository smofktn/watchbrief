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

type collectionJobResult struct {
	calls     int
	err       error
	page      domain.GenerationJobPage
	id, query string
}

func (s *collectionJobResult) List(_ context.Context, _ [32]byte, id, query string) (domain.GenerationJobPage, error) {
	s.calls++
	s.id, s.query = id, query
	return s.page, s.err
}

func TestCollectionGenerationJobHTTPBoundary(t *testing.T) {
	valid := SessionCookieName + "=" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	for _, tc := range []struct {
		name, cookie, path string
		err                error
		status             int
		code               string
	}{
		{"empty", valid, "/api/collections/01ARZ3NDEKTSV4RRFFQ69G5FAV/generation-jobs", nil, 200, `"generation_jobs":[]`},
		{"unauthenticated before invalid query", "", "/api/collections/01ARZ3NDEKTSV4RRFFQ69G5FAV/generation-jobs?cursor=%xx", nil, 401, "unauthenticated"},
		{"unknown session", valid, "/api/collections/01ARZ3NDEKTSV4RRFFQ69G5FAV/generation-jobs?cursor=%xx", domain.ErrUnauthenticated, 401, "unauthenticated"},
		{"bad ID or query", valid, "/api/collections/bad/generation-jobs", domain.ErrInvalidIdentifier, 400, "invalid_request"},
		{"bad cursor", valid, "/api/collections/01ARZ3NDEKTSV4RRFFQ69G5FAV/generation-jobs?cursor=bad", domain.ErrGenerationJobCursor, 400, "invalid_request"},
		{"not found", valid, "/api/collections/01ARZ3NDEKTSV4RRFFQ69G5FAV/generation-jobs", domain.ErrResourceNotFound, 404, "not_found"},
		{"storage failure is sanitized", valid, "/api/collections/01ARZ3NDEKTSV4RRFFQ69G5FAV/generation-jobs", errors.New("private SQL cursor Collection data"), 500, "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			service := &collectionJobResult{err: tc.err, page: domain.GenerationJobPage{GenerationJobs: []domain.GenerationJobSummary{}}}
			h, err := NewHandler("http://127.0.0.1:5173", &loginResult{}, &currentResult{}, &logoutResult{}, slog.New(slog.NewJSONHandler(&logs, nil)))
			if err != nil {
				t.Fatal(err)
			}
			h = WithCollectionGenerationJobs(h, service, slog.New(slog.NewJSONHandler(&logs, nil)))
			r := httptest.NewRequest("GET", tc.path, nil)
			r.Header.Set("Cookie", tc.cookie)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("wrong response classification")
			}
			if strings.Contains(w.Body.String(), "private SQL") || strings.Contains(logs.String(), "private SQL") || strings.Contains(logs.String(), "cursor Collection data") {
				t.Fatal("private failure details exposed")
			}
			if tc.cookie == "" && service.calls != 0 {
				t.Fatal("unauthenticated request reached service")
			}
			if tc.name == "empty" && !strings.Contains(w.Body.String(), `"next_cursor":null`) {
				t.Fatal("empty list contract missing")
			}
		})
	}
}
