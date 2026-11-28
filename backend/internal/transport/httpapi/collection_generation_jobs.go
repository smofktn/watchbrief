package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func WithCollectionGenerationJobs(handler http.Handler, service CollectionGenerationJobService, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api/collections/{id}/generation-jobs", collectionGenerationJobsHandler(service, logger))
	mux.Handle("/", handler)
	return mux
}

type CollectionGenerationJobService interface {
	List(context.Context, [32]byte, string, string) (domain.GenerationJobPage, error)
}

func collectionGenerationJobsHandler(service CollectionGenerationJobService, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, digest, ok := sessionToken(r)
		if !ok {
			writeError(w, 401, "unauthenticated", "ログインしてください。")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		value, err := service.List(ctx, digest, r.PathValue("id"), r.URL.RawQuery)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrUnauthenticated):
				writeError(w, 401, "unauthenticated", "ログインしてください。")
			case errors.Is(err, domain.ErrInvalidIdentifier), errors.Is(err, domain.ErrGenerationJobCursor):
				writeError(w, 400, "invalid_request", "取得条件を確認してください。")
			case errors.Is(err, domain.ErrResourceNotFound):
				writeError(w, 404, "not_found", "見つかりません。")
			default:
				logger.Error("collection_generation_jobs_read_failed", "operation", "collection_generation_jobs_read", "stage", "lookup", "result", "failed", "version", "issue-90")
				writeError(w, 500, "internal_error", "処理に失敗しました。時間をおいて再度お試しください")
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(value)
	})
}
