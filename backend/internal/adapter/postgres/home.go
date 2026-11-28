package postgres

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres/generated"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func (s *UserStore) ListHomeCollections(ctx context.Context, userID string, start, end time.Time) ([]domain.HomeCollection, error) {
	rows, err := s.queries.ListHomeCollections(ctx, generated.ListHomeCollectionsParams{
		UserID: userID, StartAt: pgtype.Timestamptz{Time: start, Valid: true}, EndAt: pgtype.Timestamptz{Time: end, Valid: true},
	})
	if err != nil {
		return nil, domain.ErrHomeStore
	}
	return homeCollections(rows)
}

func homeCollections(rows []generated.ListHomeCollectionsRow) ([]domain.HomeCollection, error) {
	collections := make([]domain.HomeCollection, 0, len(rows))
	for _, row := range rows {
		if !domain.ValidIdentifier(row.ID) || row.Name == "" || row.Version < 1 {
			return nil, domain.ErrHomeStore
		}
		var reportID *string
		if row.LatestReportID != "" {
			value := row.LatestReportID
			if !domain.ValidIdentifier(value) {
				return nil, domain.ErrHomeStore
			}
			reportID = &value
		}
		counts := domain.HomeJobCounts{Queued: row.TodayQueuedCount, Running: row.TodayRunningCount, Succeeded: row.TodaySucceededCount, Failed: row.TodayFailedCount}
		if counts.Queued < 0 || counts.Running < 0 || counts.Succeeded < 0 || counts.Failed < 0 {
			return nil, domain.ErrHomeStore
		}
		collections = append(collections, domain.HomeCollection{ID: row.ID, Name: row.Name, Version: strconv.FormatInt(row.Version, 10), LatestReportID: reportID, TodayJobCounts: counts})
	}
	return collections, nil
}
