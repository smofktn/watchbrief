package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type CollectionUpdater interface {
	Update(context.Context, [32]byte, string, string, domain.CollectionUpdate) (domain.CollectionUpdateResult, error)
}

func collectionUpdateHandler(service CollectionUpdater, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, digest, ok := sessionToken(r)
		if !ok {
			writeError(w, 401, "unauthenticated", "ログインしてください。")
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" || len(r.Header.Values("Idempotency-Key")) != 1 || !domain.ValidCreationKey(r.Header.Get("Idempotency-Key")) {
			writeError(w, 400, "invalid_request", "入力内容と更新要求を確認してください。")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, 400, "invalid_request", "入力内容を確認してください。")
			return
		}
		input, err := readCollectionUpdate(body)
		if err != nil {
			writeError(w, 400, "invalid_request", "入力内容を確認してください。")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		result, err := service.Update(ctx, digest, r.PathValue("id"), r.Header.Get("Idempotency-Key"), input)
		if err != nil {
			var invalid *domain.CollectionInputError
			switch {
			case errors.Is(err, domain.ErrUnauthenticated):
				writeError(w, 401, "unauthenticated", "ログインしてください。")
			case errors.Is(err, domain.ErrResourceNotFound):
				writeError(w, 404, "not_found", "見つかりません。")
			case errors.As(err, &invalid), errors.Is(err, domain.ErrInvalidIdentifier):
				writeError(w, 400, "invalid_request", "入力内容と更新要求を確認してください。")
			case errors.Is(err, domain.ErrUpdateConflict):
				writeError(w, 409, "update_conflict", "別の更新が保存されています。最新値を確認してください。")
			case errors.Is(err, domain.ErrUpdateRequestConflict):
				writeError(w, 409, "update_request_conflict", "同じ更新要求の内容が一致しません。詳細で保存結果を確認してください。")
			case errors.Is(err, domain.ErrCollectionNotStopped):
				writeError(w, 409, "collection_not_stopped", "停止中のコレクションだけ編集できます。")
			default:
				logger.Error("collection_update_failed", "operation", "collection_update", "stage", "authenticate_or_save", "result", "failed", "version", "issue-74")
				writeError(w, 500, "internal_error", "保存結果を確認できません。同じ内容で再試行してください。")
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(result)
	})
}

func readCollectionUpdate(body []byte) (domain.CollectionUpdate, error) {
	var input domain.CollectionUpdate
	invalid := errors.New("invalid collection update JSON")
	if !utf8.Valid(body) || !validJSONUnicode(body) {
		return input, invalid
	}
	d := json.NewDecoder(bytes.NewReader(body))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return input, invalid
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		name, ok := token.(string)
		if _, duplicate := fields[name]; err != nil || !ok || duplicate {
			return input, invalid
		}
		var raw json.RawMessage
		if d.Decode(&raw) != nil {
			return input, invalid
		}
		fields[name] = raw
	}
	if t, err = d.Token(); err != nil || t != json.Delim('}') || d.Decode(new(any)) != io.EOF || len(fields) != 5 {
		return input, invalid
	}
	if json.Unmarshal(fields["version"], &input.Version) != nil || !domain.ValidCollectionVersion(input.Version) {
		return input, invalid
	}
	delete(fields, "version")
	raw, err := json.Marshal(fields)
	if err != nil {
		return input, invalid
	}
	input.NewCollection, err = readNewCollection(raw)
	return input, err
}
