package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres/generated"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type collectionJobRow struct {
	exists bool
	id     pgtype.Text
	name   pgtype.Text
	at     pgtype.Timestamptz
	status pgtype.Text
	report pgtype.Text
}

func collectionJobSummaries(rows []collectionJobRow) (bool, []domain.GenerationJobSummary, error) {
	if len(rows) == 0 {
		return false, nil, domain.ErrGenerationJobStore
	}
	exists := rows[0].exists
	for _, row := range rows {
		if row.exists != exists {
			return false, nil, domain.ErrGenerationJobStore
		}
	}
	if !rows[0].id.Valid {
		if len(rows) != 1 || rows[0].name.Valid || rows[0].at.Valid || rows[0].status.Valid || rows[0].report.Valid {
			return false, nil, domain.ErrGenerationJobStore
		}
		if !exists {
			return false, nil, nil
		}
		return true, []domain.GenerationJobSummary{}, nil
	}
	values := make([]domain.GenerationJobSummary, 0, len(rows))
	for _, row := range rows {
		if !exists {
			return false, nil, domain.ErrGenerationJobStore
		}
		if !row.id.Valid || !row.name.Valid || !row.status.Valid {
			return false, nil, domain.ErrGenerationJobStore
		}
		value, err := jobSummary(row.id.String, row.name.String, row.status.String, row.at, row.report)
		if err != nil {
			return false, nil, err
		}
		values = append(values, value)
	}
	return true, values, nil
}

func (s *UserStore) ListOwnedCollectionGenerationJobs(ctx context.Context, userID, collectionID string) (bool, []domain.GenerationJobSummary, error) {
	rows, err := s.queries.ListOwnedCollectionGenerationJobs(ctx, generated.ListOwnedCollectionGenerationJobsParams{CollectionID: collectionID, UserID: userID})
	if err != nil {
		return false, nil, domain.ErrGenerationJobStore
	}
	return collectionJobRows(rows)
}

func (s *UserStore) ListOwnedCollectionGenerationJobsBefore(ctx context.Context, userID, collectionID string, cursor domain.GenerationJobCursor) (bool, []domain.GenerationJobSummary, error) {
	rows, err := s.queries.ListOwnedCollectionGenerationJobsBefore(ctx, generated.ListOwnedCollectionGenerationJobsBeforeParams{
		CollectionID: collectionID, UserID: userID,
		BeforeAt: pgtype.Timestamptz{Time: cursor.ScheduledAt, Valid: true}, BeforeID: cursor.ID,
	})
	if err != nil {
		return false, nil, domain.ErrGenerationJobStore
	}
	return collectionJobRowsBefore(rows)
}

func collectionJobRows(rows []generated.ListOwnedCollectionGenerationJobsRow) (bool, []domain.GenerationJobSummary, error) {
	values := make([]collectionJobRow, len(rows))
	for i, row := range rows {
		values[i] = collectionJobRow{
			exists: row.CollectionExists, id: row.ID, name: row.CollectionName,
			at: row.ScheduledAt, status: row.Status, report: row.ReportID,
		}
	}
	return collectionJobSummaries(values)
}

func collectionJobRowsBefore(rows []generated.ListOwnedCollectionGenerationJobsBeforeRow) (bool, []domain.GenerationJobSummary, error) {
	values := make([]collectionJobRow, len(rows))
	for i, row := range rows {
		values[i] = collectionJobRow{
			exists: row.CollectionExists, id: row.ID, name: row.CollectionName,
			at: row.ScheduledAt, status: row.Status, report: row.ReportID,
		}
	}
	return collectionJobSummaries(values)
}
