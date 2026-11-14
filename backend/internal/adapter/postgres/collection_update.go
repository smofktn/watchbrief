package postgres

import (
	"bytes"
	"context"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres/generated"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func (s *UserStore) UpdateOwnedCollection(ctx context.Context, owner, id, key string, digest [32]byte, input domain.CollectionUpdate) (domain.CollectionUpdateResult, error) {
	var empty domain.CollectionUpdateResult
	beginner, ok := s.db.(interface {
		BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
	})
	if !ok {
		return empty, domain.ErrCollectionStore
	}
	tx, err := beginner.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return empty, domain.ErrCollectionStore
	}
	defer tx.Rollback(context.Background())
	q := generated.New(tx)
	// Share the existing per-owner creation lock: request keys across different
	// Collections cannot race, and creation order remains unchanged.
	if q.LockCollectionCreation(ctx, owner) != nil {
		return empty, domain.ErrCollectionStore
	}
	state, err := q.CollectionUpdateState(ctx, generated.CollectionUpdateStateParams{ID: id, UserID: owner})
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, domain.ErrResourceNotFound
	}
	if err != nil {
		return empty, domain.ErrCollectionStore
	}
	saved, err := q.CollectionUpdateRequest(ctx, generated.CollectionUpdateRequestParams{UserID: owner, RequestKey: key})
	if err == nil {
		if !saved.CollectionID.Valid || saved.CollectionID.String != id || !bytes.Equal(saved.InputDigest, digest[:]) {
			return empty, domain.ErrUpdateRequestConflict
		}
		return domain.CollectionUpdateResult{ID: id, Version: strconv.FormatInt(saved.ResultVersion, 10)}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return empty, domain.ErrCollectionStore
	}
	if state.Enabled {
		return empty, domain.ErrCollectionNotStopped
	}
	expected, err := strconv.ParseInt(input.Version, 10, 64)
	if err != nil || expected < 1 {
		return empty, domain.ErrInvalidIdentifier
	}
	if state.Version != expected {
		return empty, domain.ErrUpdateConflict
	}
	days := pgtype.Int4{}
	if input.InitialLookbackDays != nil {
		days = pgtype.Int4{Int32: *input.InitialLookbackDays, Valid: true}
	}
	version, err := q.UpdateOwnedCollection(ctx, generated.UpdateOwnedCollectionParams{
		ID: id, UserID: owner, Name: input.Name, ResearchInstruction: input.ResearchInstruction,
		InitialLookbackMode: input.InitialLookbackMode, InitialLookbackDays: days, Version: expected,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, domain.ErrUpdateConflict
	}
	if err != nil {
		return empty, domain.ErrCollectionStore
	}
	if q.RecordCollectionUpdate(ctx, generated.RecordCollectionUpdateParams{UserID: owner, RequestKey: key, CollectionID: pgtype.Text{String: id, Valid: true}, InputDigest: digest[:], ResultVersion: version}) != nil {
		return empty, domain.ErrCollectionStore
	}
	if tx.Commit(ctx) != nil {
		return empty, domain.ErrCollectionStore
	}
	return domain.CollectionUpdateResult{ID: id, Version: strconv.FormatInt(version, 10)}, nil
}
