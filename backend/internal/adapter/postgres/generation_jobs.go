package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres/generated"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func jobSummary(id, name, status string, at pgtype.Timestamptz, report pgtype.Text) (domain.GenerationJobSummary, error) {
	if !at.Valid || at.InfinityModifier != pgtype.Finite || at.Time.Year() < 1 || at.Time.Year() > 9999 || !domain.ValidIdentifier(id) || (status != "queued" && status != "running" && status != "succeeded" && status != "failed") || (status == "succeeded") != report.Valid || report.Valid && !domain.ValidIdentifier(report.String) {
		return domain.GenerationJobSummary{}, domain.ErrGenerationJobStore
	}
	value := domain.GenerationJobSummary{ID: id, CollectionName: name, ScheduledAt: at.Time.UTC(), Status: status}
	if report.Valid {
		value.ReportID = &report.String
	}
	return value, nil
}
func (s *UserStore) ListOwnedGenerationJobs(ctx context.Context, userID string) ([]domain.GenerationJobSummary, error) {
	rows, err := s.queries.ListOwnedGenerationJobs(ctx, userID)
	if err != nil {
		return nil, domain.ErrGenerationJobStore
	}
	values := make([]domain.GenerationJobSummary, 0, len(rows))
	for _, row := range rows {
		value, err := jobSummary(row.ID, row.CollectionName, row.Status, row.ScheduledAt, row.ReportID)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}
func (s *UserStore) ListOwnedGenerationJobsBefore(ctx context.Context, userID string, cursor domain.GenerationJobCursor) ([]domain.GenerationJobSummary, error) {
	rows, err := s.queries.ListOwnedGenerationJobsBefore(ctx, generated.ListOwnedGenerationJobsBeforeParams{UserID: userID, BeforeAt: pgtype.Timestamptz{Time: cursor.ScheduledAt, Valid: true}, BeforeID: cursor.ID})
	if err != nil {
		return nil, domain.ErrGenerationJobStore
	}
	values := make([]domain.GenerationJobSummary, 0, len(rows))
	for _, row := range rows {
		value, err := jobSummary(row.ID, row.CollectionName, row.Status, row.ScheduledAt, row.ReportID)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}
