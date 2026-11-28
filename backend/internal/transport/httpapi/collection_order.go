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

type CollectionOrderUpdater interface {
	Reorder(context.Context, [32]byte, string, domain.CollectionOrderInput) error
}

func collectionOrderHandler(service CollectionOrderUpdater, logger *slog.Logger) http.Handler {
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
		r.Body = http.MaxBytesReader(w, r.Body, 1048576)
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, 400, "invalid_request", "入力内容を確認してください。")
			return
		}
		input, err := readCollectionOrder(body)
		if err != nil || input.Validate() != nil {
			writeError(w, 400, "invalid_request", "入力内容を確認してください。")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := service.Reorder(ctx, digest, r.Header.Get("Idempotency-Key"), input); err != nil {
			switch {
			case errors.Is(err, domain.ErrUnauthenticated):
				writeError(w, 401, "unauthenticated", "ログインしてください。")
			case errors.Is(err, domain.ErrCollectionOrderInput), errors.Is(err, domain.ErrInvalidIdentifier):
				writeError(w, 400, "invalid_request", "入力内容を確認してください。")
			case errors.Is(err, domain.ErrCollectionOrderConflict):
				writeError(w, 409, "order_conflict", "Collectionの構成または設定が更新されています。最新状態を確認してください。")
			case errors.Is(err, domain.ErrCollectionOrderRequestConflict):
				writeError(w, 409, "order_request_conflict", "同じ更新要求の内容が一致しません。現在の順序を確認してください。")
			default:
				logger.Error("collection_order_failed", "operation", "collection_order", "stage", "save", "result", "failed", "version", "issue-96")
				writeError(w, 500, "internal_error", "保存結果を確認できません。同じ内容で再試行してください。")
			}
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	})
}

func readCollectionOrder(body []byte) (domain.CollectionOrderInput, error) {
	var input domain.CollectionOrderInput
	invalid := errors.New("invalid collection order JSON")
	if !utf8.Valid(body) || !validJSONUnicode(body) {
		return input, invalid
	}
	d := json.NewDecoder(bytes.NewReader(body))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return input, invalid
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		name, ok := token.(string)
		if err != nil || !ok {
			return input, invalid
		}
		if _, duplicate := fields[name]; duplicate {
			return input, invalid
		}
		var raw json.RawMessage
		if d.Decode(&raw) != nil {
			return input, invalid
		}
		fields[name] = raw
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') || d.Decode(new(any)) != io.EOF || len(fields) != 2 {
		return input, invalid
	}
	if err := decodeExpectedOrder(fields["expected"], &input.Expected); err != nil {
		return input, invalid
	}
	if err := json.Unmarshal(fields["ordered_ids"], &input.OrderedIDs); err != nil || input.OrderedIDs == nil {
		return input, invalid
	}
	return input, nil
}

func decodeExpectedOrder(raw []byte, entries *[]domain.CollectionOrderEntry) error {
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil || rows == nil {
		return errors.New("invalid expected order")
	}
	for _, row := range rows {
		fields, err := strictOrderObject(row)
		if err != nil || len(fields) != 2 {
			return errors.New("invalid expected row")
		}
		var entry domain.CollectionOrderEntry
		id, hasID := fields["id"]
		version, hasVersion := fields["version"]
		if !hasID || !hasVersion || json.Unmarshal(id, &entry.ID) != nil || json.Unmarshal(version, &entry.Version) != nil {
			return errors.New("invalid expected values")
		}
		*entries = append(*entries, entry)
	}
	return nil
}

func strictOrderObject(raw []byte) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("invalid order object")
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		name, ok := token.(string)
		if err != nil || !ok {
			return nil, errors.New("invalid order key")
		}
		if _, duplicate := fields[name]; duplicate {
			return nil, errors.New("duplicate order key")
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, errors.New("invalid order value")
		}
		fields[name] = value
	}
	if token, err = d.Token(); err != nil || token != json.Delim('}') || d.Decode(new(any)) != io.EOF {
		return nil, errors.New("invalid order object end")
	}
	return fields, nil
}
