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

type collectionResult struct {
	calls int
	err   error
}

func (s *collectionResult) List(context.Context, [32]byte) ([]domain.CollectionSummary, error) {
	s.calls++
	return []domain.CollectionSummary{}, s.err
}
func (s *collectionResult) Get(context.Context, [32]byte, string) (domain.Collection, error) {
	s.calls++
	return domain.Collection{CollectionSummary: domain.CollectionSummary{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}, Targets: []domain.WatchTarget{}}, s.err
}

func TestCollectionHTTPBoundary(t *testing.T) {
	valid := SessionCookieName + "=" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	for _, tc := range []struct {
		name, cookie, path string
		err                error
		status             int
		code               string
	}{
		{"empty list", valid, "/api/collections", nil, 200, "collections"},
		{"detail", valid, "/api/collections/01ARZ3NDEKTSV4RRFFQ69G5FAV", nil, 200, "targets"},
		{"no cookie", "", "/api/collections", nil, 401, "unauthenticated"},
		{"duplicate cookie", valid + "; " + valid, "/api/collections", nil, 401, "unauthenticated"},
		{"invalid cookie", SessionCookieName + "=invalid", "/api/collections", nil, 401, "unauthenticated"},
		{"unknown session", valid, "/api/collections", domain.ErrUnauthenticated, 401, "unauthenticated"},
		{"not found", valid, "/api/collections/01ARZ3NDEKTSV4RRFFQ69G5FAV", domain.ErrResourceNotFound, 404, "not_found"},
		{"invalid ID", valid, "/api/collections/invalid", domain.ErrInvalidIdentifier, 400, "invalid_request"},
		{"DB failure", valid, "/api/collections", errors.New("private SQL connection data"), 500, "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			service := &collectionResult{err: tc.err}
			h, err := NewHandler("http://127.0.0.1:5173", &loginResult{}, &currentResult{}, &logoutResult{}, slog.New(slog.NewJSONHandler(&logs, nil)), service)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("GET", tc.path, nil)
			r.Header.Set("Cookie", tc.cookie)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("wrong resource classification")
			}
			if strings.Contains(w.Body.String(), "private SQL") || strings.Contains(logs.String(), "private SQL") || strings.Contains(logs.String(), valid) {
				t.Fatal("private value exposed")
			}
			if tc.name == "no cookie" || tc.name == "duplicate cookie" || tc.name == "invalid cookie" {
				if service.calls != 0 {
					t.Fatal("invalid Cookie reached service")
				}
			}
			if tc.name == "empty list" && !strings.Contains(w.Body.String(), `"collections":[]`) {
				t.Fatal("empty result must be array")
			}
		})
	}
}
