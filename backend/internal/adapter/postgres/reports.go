package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres/generated"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func (s *UserStore) ListOwnedReports(ctx context.Context, userID string) ([]domain.ReportSummary, error) {
	rows, err := s.queries.ListOwnedReports(ctx, userID)
	if err != nil {
		return nil, domain.ErrReportStore
	}
	values := make([]domain.ReportSummary, 0, len(rows))
	for _, row := range rows {
		values = append(values, domain.ReportSummary{ID: row.ID, CollectionName: row.CollectionName, ScheduledAt: row.ScheduledAt.Time, GeneratedAt: row.GeneratedAt.Time})
	}
	return values, nil
}
func (s *UserStore) OwnedReport(ctx context.Context, userID, id string) (domain.Report, error) {
	row, err := s.queries.OwnedReport(ctx, generated.OwnedReportParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Report{}, domain.ErrResourceNotFound
	}
	if err != nil {
		return domain.Report{}, domain.ErrReportStore
	}
	return domain.Report{ReportSummary: domain.ReportSummary{ID: row.ID, CollectionName: row.CollectionName, ScheduledAt: row.ScheduledAt.Time, GeneratedAt: row.GeneratedAt.Time}, BodyMarkdown: row.BodyMarkdown, SummaryMarkdown: row.SummaryMarkdown, UpdatesStartedAt: row.UpdatesStartedAt.Time, UpdatesEndedAt: row.UpdatesEndedAt.Time}, nil
}
