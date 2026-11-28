package postgres

import (
	"bytes"
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres/generated"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func (s *UserStore) ReorderOwnedCollections(ctx context.Context, owner, key string, digest [32]byte, input domain.CollectionOrderInput) error {
	beginner, ok := s.db.(interface {
		BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
	})
	if !ok {
		return domain.ErrCollectionStore
	}
	tx, err := beginner.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.ErrCollectionStore
	}
	defer tx.Rollback(context.Background())
	q := generated.New(tx)
	if err := q.LockCollectionOrder(ctx, owner); err != nil {
		return domain.ErrCollectionStore
	}
	saved, err := q.CollectionOrderRequest(ctx, generated.CollectionOrderRequestParams{UserID: owner, RequestKey: key})
	if err == nil {
		if !bytes.Equal(saved, digest[:]) {
			return domain.ErrCollectionOrderRequestConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return domain.ErrCollectionStore
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrCollectionStore
	}
	current, err := q.ListOwnedCollectionOrder(ctx, owner)
	if err != nil {
		return domain.ErrCollectionStore
	}
	if len(current) != len(input.Expected) {
		return domain.ErrCollectionOrderConflict
	}
	for i, row := range current {
		if row.ID != input.Expected[i].ID || strconv.FormatInt(row.Version, 10) != input.Expected[i].Version {
			return domain.ErrCollectionOrderConflict
		}
	}
	visibleOrderChanged := false
	for i, row := range current {
		if row.ID != input.OrderedIDs[i] {
			visibleOrderChanged = true
			break
		}
	}
	changed := int64(0)
	if visibleOrderChanged {
		desiredPosition := make(map[string]int32, len(input.OrderedIDs))
		for i, id := range input.OrderedIDs {
			desiredPosition[id] = int32(i)
		}
		for _, row := range current {
			if row.DisplayOrder != desiredPosition[row.ID] {
				changed++
			}
		}
	}
	if changed > 0 {
		updated, err := q.UpdateOwnedCollectionOrder(ctx, generated.UpdateOwnedCollectionOrderParams{OrderedIds: input.OrderedIDs, UserID: owner})
		if err != nil || updated != changed {
			return domain.ErrCollectionStore
		}
	}
	if err := q.RecordCollectionOrderRequest(ctx, generated.RecordCollectionOrderRequestParams{UserID: owner, RequestKey: key, InputDigest: digest[:]}); err != nil {
		return domain.ErrCollectionStore
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ErrCollectionStore
	}
	return nil
}
