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

func TestReadCollectionOrderRejectsDuplicateExpectedKeys(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for name, body := range map[string]string{
		"same id twice":      `{"expected":[{"id":"` + id + `","id":"` + id + `","version":"1"}],"ordered_ids":["` + id + `"]}`,
		"different id twice": `{"expected":[{"id":"` + id + `","id":"01ARZ3NDEKTSV4RRFFQ69G5FAW","version":"1"}],"ordered_ids":["` + id + `"]}`,
		"same version twice": `{"expected":[{"id":"` + id + `","version":"1","version":"1"}],"ordered_ids":["` + id + `"]}`,
		"different version":  `{"expected":[{"id":"` + id + `","version":"1","version":"2"}],"ordered_ids":["` + id + `"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readCollectionOrder([]byte(body)); err == nil {
				t.Fatal("duplicate nested JSON key accepted")
			}
		})
	}
}

type orderResult struct {
	*collectionResult
	orderCalls int
	key        string
	input      domain.CollectionOrderInput
	err        error
}

func (s *orderResult) Reorder(_ context.Context, _ [32]byte, key string, input domain.CollectionOrderInput) error {
	s.orderCalls++
	s.key, s.input = key, input
	return s.err
}

func TestCollectionOrderHTTPBoundary(t *testing.T) {
	const origin = "http://127.0.0.1:5173"
	const key = "00112233-4455-4677-8899-aabbccddeeff"
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	cookie := SessionCookieName + "=" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	const valid = `{"expected":[{"id":"` + id + `","version":"1"}],"ordered_ids":["` + id + `"]}`
	for _, tc := range []struct {
		name, body, cookie, origin string
		err                        error
		status, calls              int
		code                       string
	}{
		{"save", valid, cookie, origin, nil, 204, 1, ""},
		{"no csrf", valid, cookie, "", nil, 403, 0, "csrf_rejected"},
		{"no cookie", valid, "", origin, nil, 401, 0, "unauthenticated"},
		{"nested duplicate same", strings.Replace(valid, `,"version":"1"`, `,"id":"`+id+`","version":"1"`, 1), cookie, origin, nil, 400, 0, "invalid_request"},
		{"nested duplicate different", strings.Replace(valid, `,"version":"1"`, `,"id":"01ARZ3NDEKTSV4RRFFQ69G5FAW","version":"1"`, 1), cookie, origin, nil, 400, 0, "invalid_request"},
		{"bad set", strings.Replace(valid, `"ordered_ids":["`+id+`"]`, `"ordered_ids":[]`, 1), cookie, origin, nil, 400, 0, "invalid_request"},
		{"stale owner state", valid, cookie, origin, domain.ErrCollectionOrderConflict, 409, 1, "order_conflict"},
		{"request collision", valid, cookie, origin, domain.ErrCollectionOrderRequestConflict, 409, 1, "order_request_conflict"},
		{"database failure", valid, cookie, origin, errors.New("private database marker"), 500, 1, "internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &orderResult{collectionResult: &collectionResult{}, err: tc.err}
			var logs bytes.Buffer
			h, err := NewHandler(origin, &loginResult{}, &currentResult{}, &logoutResult{}, slog.New(slog.NewJSONHandler(&logs, nil)), service)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("PUT", "/api/collections/order", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Idempotency-Key", key)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Cookie", tc.cookie)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || service.orderCalls != tc.calls || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" {
				t.Fatal("wrong ordering boundary or side effect")
			}
			if tc.code != "" && !strings.Contains(w.Body.String(), tc.code) {
				t.Fatal("wrong order error classification")
			}
			if strings.Contains(w.Body.String()+logs.String(), "private database") || strings.Contains(logs.String(), key) || strings.Contains(logs.String(), cookie) {
				t.Fatal("private ordering input leaked")
			}
			if tc.calls == 1 && (service.key != key || len(service.input.Expected) != 1 || service.input.Expected[0].ID != id) {
				t.Fatal("authenticated ordering input was not delivered")
			}
		})
	}
}
