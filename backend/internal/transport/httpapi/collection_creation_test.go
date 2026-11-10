package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"github.com/smofktn/watchbrief/backend/internal/domain"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

type creationResult struct {
	collectionResult
	saveCalls int
	saveErr   error
	created   bool
}

func (s *creationResult) Execute(context.Context, [32]byte, string, domain.NewCollection) (string, bool, error) {
	s.saveCalls++
	return "01ARZ3NDEKTSV4RRFFQ69G5FAV", s.created, s.saveErr
}
func TestCollectionCreationBoundary(t *testing.T) {
	const body = `{"name":"架空","research_instruction":"","initial_lookback_mode":"ai","initial_lookback_days":null}`
	const key = "00112233-4455-4677-8899-aabbccddeeff"
	const origin = "http://127.0.0.1:5173"
	cookie := SessionCookieName + "=" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	for _, tc := range []struct {
		name, body, key, origin, cookie string
		err                             error
		created                         bool
		status, calls                   int
	}{
		{"create", body, key, origin, cookie, nil, true, 201, 1},
		{"replay", body, key, origin, cookie, nil, false, 200, 1},
		{"no csrf", body, key, "", cookie, nil, false, 403, 0},
		{"foreign csrf", body, key, "https://example.invalid", cookie, nil, false, 403, 0},
		{"no cookie", body, key, origin, "", nil, false, 401, 0},
		{"duplicate cookie", body, key, origin, cookie + "; " + cookie, nil, false, 401, 0},
		{"no key", body, "", origin, cookie, nil, false, 400, 0},
		{"bad key", body, "bad", origin, cookie, nil, false, 400, 0},
		{"null", "null", key, origin, cookie, nil, false, 400, 0},
		{"missing", "{}", key, origin, cookie, nil, false, 400, 0},
		{"unknown", strings.TrimSuffix(body, "}") + `,"user_id":"attacker"}`, key, origin, cookie, nil, false, 400, 0},
		{"duplicate", strings.TrimSuffix(body, "}") + `,"name":"other"}`, key, origin, cookie, nil, false, 400, 0},
		{"unpaired", strings.Replace(body, "架空", `\ud800`, 1), key, origin, cookie, nil, false, 400, 0},
		{"extra", body + "{}", key, origin, cookie, nil, false, 400, 0},
		{"too big", strings.Repeat(" ", 65537), key, origin, cookie, nil, false, 400, 0},
		{"invalid utf8", string([]byte{0xff}), key, origin, cookie, nil, false, 400, 0},
		{"invalid fields", body, key, origin, cookie, &domain.CollectionInputError{Fields: map[string]string{"name": "固定案内"}}, false, 400, 1},
		{"expired", body, key, origin, cookie, domain.ErrUnauthenticated, false, 401, 1},
		{"conflict", body, key, origin, cookie, domain.ErrCreationConflict, false, 409, 1},
		{"failure", body, key, origin, cookie, errors.New("private database detail"), false, 500, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &creationResult{saveErr: tc.err, created: tc.created}
			var logs bytes.Buffer
			h, err := NewHandler(origin, &loginResult{}, &currentResult{}, &logoutResult{}, slog.New(slog.NewJSONHandler(&logs, nil)), s)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("POST", "/api/collections", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Cookie", tc.cookie)
			r.Header.Set("Idempotency-Key", tc.key)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || s.saveCalls != tc.calls || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Set-Cookie") != "" {
				t.Fatal("wrong classification or side effect")
			}
			if strings.Contains(w.Body.String()+logs.String(), "private database") || strings.Contains(logs.String(), "架空") || strings.Contains(logs.String(), key) || strings.Contains(logs.String(), cookie) {
				t.Fatal("private data logged")
			}
			if tc.status < 300 && w.Header().Get("Location") != "/api/collections/01ARZ3NDEKTSV4RRFFQ69G5FAV" {
				t.Fatal("missing result URL")
			}
		})
	}
}
