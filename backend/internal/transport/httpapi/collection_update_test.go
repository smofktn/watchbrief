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

type updateResult struct {
	collectionResult
	calls   int
	err     error
	input   domain.CollectionUpdate
	id, key string
}

func (s *updateResult) Update(_ context.Context, _ [32]byte, id, key string, input domain.CollectionUpdate) (domain.CollectionUpdateResult, error) {
	s.calls++
	s.input = input
	s.id = id
	s.key = key
	return domain.CollectionUpdateResult{ID: id, Version: "2"}, s.err
}
func TestCollectionUpdateBoundary(t *testing.T) {
	const body = `{"name":"架空","research_instruction":"","initial_lookback_mode":"ai","initial_lookback_days":null,"version":"1"}`
	const key = "00112233-4455-4677-8899-aabbccddeeff"
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const origin = "http://127.0.0.1:5173"
	cookie := SessionCookieName + "=" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	for _, tc := range []struct {
		name, body, key, origin, cookie string
		err                             error
		status, calls                   int
	}{
		{"save", body, key, origin, cookie, nil, 200, 1},
		{"no origin", body, key, "", cookie, nil, 403, 0},
		{"foreign origin", body, key, "https://example.invalid", cookie, nil, 403, 0},
		{"no cookie", body, key, origin, "", nil, 401, 0},
		{"duplicate cookie", body, key, origin, cookie + "; " + cookie, nil, 401, 0},
		{"no key", body, "", origin, cookie, nil, 400, 0},
		{"null", "null", key, origin, cookie, nil, 400, 0},
		{"duplicate", strings.TrimSuffix(body, "}") + `,"version":"2"}`, key, origin, cookie, nil, 400, 0},
		{"unknown", strings.TrimSuffix(body, "}") + `,"enabled":true}`, key, origin, cookie, nil, 400, 0},
		{"numeric version", strings.Replace(body, `"version":"1"`, `"version":1`, 1), key, origin, cookie, nil, 400, 0},
		{"missing", strings.Replace(body, `,"version":"1"`, "", 1), key, origin, cookie, nil, 400, 0},
		{"null version", strings.Replace(body, `"version":"1"`, `"version":null`, 1), key, origin, cookie, nil, 400, 0},
		{"unpaired", strings.Replace(body, "架空", `\ud800`, 1), key, origin, cookie, nil, 400, 0},
		{"extra", body + "{}", key, origin, cookie, nil, 400, 0},
		{"too big", strings.Repeat(" ", 65537), key, origin, cookie, nil, 400, 0},
		{"invalid utf8", string([]byte{0xff}), key, origin, cookie, nil, 400, 0},
		{"expired", body, key, origin, cookie, domain.ErrUnauthenticated, 401, 1},
		{"hidden", body, key, origin, cookie, domain.ErrResourceNotFound, 404, 1},
		{"invalid ID", body, key, origin, cookie, domain.ErrInvalidIdentifier, 400, 1},
		{"invalid fields", body, key, origin, cookie, &domain.CollectionInputError{}, 400, 1},
		{"version conflict", body, key, origin, cookie, domain.ErrUpdateConflict, 409, 1},
		{"key conflict", body, key, origin, cookie, domain.ErrUpdateRequestConflict, 409, 1},
		{"enabled", body, key, origin, cookie, domain.ErrCollectionNotStopped, 409, 1},
		{"failure", body, key, origin, cookie, errors.New("private database detail"), 500, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &updateResult{err: tc.err}
			var logs bytes.Buffer
			h, err := NewHandler(origin, &loginResult{}, &currentResult{}, &logoutResult{}, slog.New(slog.NewJSONHandler(&logs, nil)), s)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("PATCH", "/api/collections/"+id, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Cookie", tc.cookie)
			r.Header.Set("Idempotency-Key", tc.key)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || s.calls != tc.calls || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" {
				t.Fatal("wrong boundary or side effect")
			}
			if strings.Contains(w.Body.String()+logs.String(), "private database") || strings.Contains(logs.String(), "架空") || strings.Contains(logs.String(), key) || strings.Contains(logs.String(), cookie) {
				t.Fatal("private data exposed")
			}
			if tc.status == 200 && (s.id != id || s.key != key || s.input.Version != "1" || s.input.Name != "架空") {
				t.Fatal("update contract changed")
			}
		})
	}
}
