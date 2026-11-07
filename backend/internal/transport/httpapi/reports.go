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

// WithReports keeps the authentication and Collection constructor contract intact.
func WithReports(handler http.Handler, service ReportService, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api/reports", reportHandler(service, logger, false))
	mux.Handle("GET /api/reports/{id}", reportHandler(service, logger, true))
	mux.Handle("/", handler)
	return mux
}

type ReportService interface {
	List(context.Context, [32]byte) ([]domain.ReportSummary, error)
	Get(context.Context, [32]byte, string) (domain.Report, error)
}

func reportHandler(service ReportService, logger *slog.Logger, detail bool) http.Handler {
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
			var rows []domain.ReportSummary
			rows, err = service.List(ctx, digest)
			value = struct {
				Reports []domain.ReportSummary `json:"reports"`
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
				logger.Error("report_read_failed", "operation", "report_read", "stage", "lookup", "result", "failed", "version", "issue-57")
				writeError(w, 500, "internal_error", "処理に失敗しました。時間をおいて再度お試しください")
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(value)
	})
}
