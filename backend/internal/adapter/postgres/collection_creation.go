package postgres

import (
	"bytes"
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres/generated"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func (s *UserStore) CreateOwnedCollection(ctx context.Context, owner, key string, digest [32]byte, id string, input domain.NewCollection) (string, bool, error) {
	beginner, ok := s.db.(interface {
		BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
	})
	if !ok {
		return "", false, domain.ErrCollectionStore
	}
	tx, err := beginner.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return "", false, domain.ErrCollectionStore
	}
	defer tx.Rollback(context.Background())
	q := generated.New(tx)
	if err := q.LockCollectionCreation(ctx, owner); err != nil {
		return "", false, domain.ErrCollectionStore
	}
	existing, err := q.CollectionCreationRequest(ctx, generated.CollectionCreationRequestParams{UserID: owner, RequestKey: key})
	if err == nil {
		if !bytes.Equal(existing.InputDigest, digest[:]) || !existing.CollectionID.Valid {
			return "", false, domain.ErrCreationConflict
		}
		return existing.CollectionID.String, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, domain.ErrCollectionStore
	}
	days := pgtype.Int4{}
	if input.InitialLookbackDays != nil {
		days = pgtype.Int4{Int32: *input.InitialLookbackDays, Valid: true}
	}
	err = q.AppendOwnedCollection(ctx, generated.AppendOwnedCollectionParams{
		ID: id, UserID: owner, Name: input.Name, ResearchInstruction: input.ResearchInstruction,
		InitialLookbackMode: input.InitialLookbackMode, InitialLookbackDays: days,
	})
	if err != nil {
		return "", false, domain.ErrCollectionStore
	}
	err = q.RecordCollectionCreation(ctx, generated.RecordCollectionCreationParams{UserID: owner, RequestKey: key, InputDigest: digest[:], CollectionID: pgtype.Text{String: id, Valid: true}})
	if err != nil {
		return "", false, domain.ErrCollectionStore
	}
	if err := tx.Commit(ctx); err != nil {
		return "", false, domain.ErrCollectionStore
	}
	return id, true, nil
}
