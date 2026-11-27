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

// WithGenerationJobs keeps the authentication and Collection constructor contract intact.
func WithGenerationJobs(handler http.Handler, service GenerationJobService, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api/generation-jobs", generationJobsHandler(service, logger))
	mux.Handle("/", handler)
	return mux
}

type GenerationJobService interface {
	List(context.Context, [32]byte, string) (domain.GenerationJobPage, error)
}

func generationJobsHandler(service GenerationJobService, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, digest, ok := sessionToken(r)
		if !ok {
			writeError(w, 401, "unauthenticated", "ログインしてください。")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		value, err := service.List(ctx, digest, r.URL.RawQuery)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrUnauthenticated):
				writeError(w, 401, "unauthenticated", "ログインしてください。")
			case errors.Is(err, domain.ErrGenerationJobCursor):
				writeError(w, 400, "invalid_request", "取得条件を確認してください。")
			default:
				logger.Error("generation_jobs_read_failed", "operation", "generation_jobs_read", "stage", "lookup", "result", "failed", "version", "issue-88")
				writeError(w, 500, "internal_error", "処理に失敗しました。時間をおいて再度お試しください")
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(value)
	})
}
