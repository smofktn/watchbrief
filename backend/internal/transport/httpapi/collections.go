package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type CollectionService interface {
	List(context.Context, [32]byte) ([]domain.CollectionSummary, error)
	Get(context.Context, [32]byte, string) (domain.Collection, error)
}

func collectionHandler(service CollectionService, logger *slog.Logger, detail bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, digest, ok := sessionToken(r)
		if !ok {
			writeError(w, 401, "unauthenticated", "ログインしてください。")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		var value any
		var err error
		if detail {
			value, err = service.Get(ctx, digest, r.PathValue("id"))
		} else {
			var rows []domain.CollectionSummary
			rows, err = service.List(ctx, digest)
			value = struct {
				Collections []domain.CollectionSummary `json:"collections"`
			}{rows}
		}
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrUnauthenticated):
				writeError(w, 401, "unauthenticated", "ログインしてください。")
			case errors.Is(err, domain.ErrInvalidIdentifier):
				writeError(w, 400, "invalid_request", "IDを確認してください。")
			case errors.Is(err, domain.ErrResourceNotFound):
				writeError(w, 404, "not_found", "見つかりません。")
			default:
				logger.Error("collection_read_failed", "operation", "collection_read", "stage", "lookup", "result", "failed", "version", "issue-56")
				writeError(w, 500, "internal_error", "処理に失敗しました。時間をおいて再度お試しください")
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(value)
	})
}
