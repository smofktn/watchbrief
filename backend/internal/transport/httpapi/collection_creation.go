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

type CollectionCreator interface {
	Execute(context.Context, [32]byte, string, domain.NewCollection) (string, bool, error)
}

func collectionCreationHandler(service CollectionCreator, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, digest, ok := sessionToken(r)
		if !ok {
			writeError(w, 401, "unauthenticated", "ログインしてください。")
			return
		}
		media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || media != "application/json" || len(r.Header.Values("Idempotency-Key")) != 1 || !domain.ValidCreationKey(r.Header.Get("Idempotency-Key")) {
			writeError(w, 400, "invalid_request", "入力内容と作成要求を確認してください。")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, 400, "invalid_request", "入力内容を確認してください。")
			return
		}
		input, err := readNewCollection(body)
		if err != nil {
			writeError(w, 400, "invalid_request", "入力内容を確認してください。")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		id, created, err := service.Execute(ctx, digest, r.Header.Get("Idempotency-Key"), input)
		if err != nil {
			var invalid *domain.CollectionInputError
			switch {
			case errors.Is(err, domain.ErrUnauthenticated):
				writeError(w, 401, "unauthenticated", "ログインしてください。")
			case errors.As(err, &invalid):
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "no-store")
				w.WriteHeader(400)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "invalid_request", "message": "入力内容を確認してください。", "fields": invalid.Fields}})
			case errors.Is(err, domain.ErrInvalidIdentifier):
				writeError(w, 400, "invalid_request", "作成要求を確認してください。")
			case errors.Is(err, domain.ErrCreationConflict):
				writeError(w, 409, "creation_conflict", "作成要求を確認できません。一覧で保存結果を確認してください。")
			default:
				logger.Error("collection_create_failed", "operation", "collection_create", "stage", "authenticate_or_save", "result", "failed", "version", "issue-18")
				writeError(w, 500, "internal_error", "保存結果を確認できません。同じ内容で再試行してください。")
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Location", "/api/collections/"+id)
		status := 200
		if created {
			status = 201
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(struct {
			ID string `json:"id"`
		}{id})
	})
}

// Strict object decoding prevents unknown, duplicate, null and lossy Unicode input.
func readNewCollection(body []byte) (domain.NewCollection, error) {
	invalid := errors.New("invalid collection JSON")
	var input domain.NewCollection
	if !utf8.Valid(body) || !validJSONUnicode(body) {
		return input, invalid
	}
	d := json.NewDecoder(bytes.NewReader(body))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return input, invalid
	}
	fields := map[string]bool{}
	for d.More() {
		token, err := d.Token()
		name, ok := token.(string)
		if err != nil || !ok || fields[name] {
			return input, invalid
		}
		fields[name] = true
		if name == "initial_lookback_days" {
			if err := d.Decode(&input.InitialLookbackDays); err != nil {
				return input, invalid
			}
			continue
		}
		var value *string
		if err := d.Decode(&value); err != nil || value == nil {
			return input, invalid
		}
		switch name {
		case "name":
			input.Name = *value
		case "research_instruction":
			input.ResearchInstruction = *value
		case "initial_lookback_mode":
			input.InitialLookbackMode = *value
		default:
			return input, invalid
		}
	}
	if t, err = d.Token(); err != nil || t != json.Delim('}') || d.Decode(new(any)) != io.EOF || len(fields) != 4 {
		return input, invalid
	}
	return input, nil
}
