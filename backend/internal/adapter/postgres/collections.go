package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres/generated"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func (s *UserStore) UserIDBySessionHash(ctx context.Context, digest [32]byte) (string, error) {
	id, err := s.queries.UserIDBySessionHash(ctx, digest[:])
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrUnauthenticated
	}
	if err != nil {
		return "", domain.ErrSessionStore
	}
	return id, nil
}

func (s *UserStore) ListOwnedCollections(ctx context.Context, userID string) ([]domain.CollectionSummary, error) {
	rows, err := s.queries.ListOwnedCollections(ctx, userID)
	if err != nil {
		return nil, domain.ErrCollectionStore
	}
	values := make([]domain.CollectionSummary, 0, len(rows))
	for _, row := range rows {
		values = append(values, domain.CollectionSummary{ID: row.ID, Name: row.Name, Enabled: row.Enabled, DisplayOrder: row.DisplayOrder})
	}
	return values, nil
}

func (s *UserStore) OwnedCollection(ctx context.Context, userID, id string) (domain.Collection, error) {
	document, err := s.queries.OwnedCollectionDocument(ctx, generated.OwnedCollectionDocumentParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Collection{}, domain.ErrResourceNotFound
	}
	if err != nil {
		return domain.Collection{}, domain.ErrCollectionStore
	}
	var value domain.Collection
	if json.Unmarshal(document, &value) != nil {
		return domain.Collection{}, domain.ErrCollectionStore
	}
	return value, nil
}
