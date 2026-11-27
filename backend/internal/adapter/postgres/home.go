package postgres

import (
	"context"

	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres/generated"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func (s *UserStore) ListHomeCollections(ctx context.Context, userID string) ([]domain.HomeCollection, error) {
	rows, err := s.queries.ListHomeCollections(ctx, userID)
	if err != nil {
		return nil, domain.ErrHomeStore
	}
	return homeCollections(rows)
}

func homeCollections(rows []generated.ListHomeCollectionsRow) ([]domain.HomeCollection, error) {
	collections := make([]domain.HomeCollection, 0, len(rows))
	for _, row := range rows {
		if !domain.ValidIdentifier(row.ID) || row.Name == "" {
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
		collections = append(collections, domain.HomeCollection{ID: row.ID, Name: row.Name, LatestReportID: reportID})
	}
	return collections, nil
}
