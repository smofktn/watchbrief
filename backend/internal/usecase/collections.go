package usecase

import (
	"context"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type CollectionStore interface {
	UserIDBySessionHash(context.Context, [32]byte) (string, error)
	ListOwnedCollections(context.Context, string) ([]domain.CollectionSummary, error)
	OwnedCollection(context.Context, string, string) (domain.Collection, error)
}

type Collections struct {
	Store   CollectionStore
	Creator *CreateCollection
}

func (uc Collections) Execute(ctx context.Context, digest [32]byte, key string, input domain.NewCollection) (string, bool, error) {
	if uc.Creator == nil {
		return "", false, domain.ErrCollectionStore
	}
	return uc.Creator.Execute(ctx, digest, key, input)
}

func (uc Collections) List(ctx context.Context, digest [32]byte) ([]domain.CollectionSummary, error) {
	if ctx.Err() != nil || uc.Store == nil {
		return nil, domain.ErrCollectionStore
	}
	userID, err := uc.Store.UserIDBySessionHash(ctx, digest)
	if err != nil {
		return nil, err
	}
	rows, err := uc.Store.ListOwnedCollections(ctx, userID)
	if ctx.Err() != nil {
		return nil, domain.ErrCollectionStore
	}
	return rows, err
}

func (uc Collections) Get(ctx context.Context, digest [32]byte, id string) (domain.Collection, error) {
	if ctx.Err() != nil || uc.Store == nil {
		return domain.Collection{}, domain.ErrCollectionStore
	}
	userID, err := uc.Store.UserIDBySessionHash(ctx, digest)
	if err != nil {
		return domain.Collection{}, err
	}
	if !domain.ValidIdentifier(id) {
		return domain.Collection{}, domain.ErrInvalidIdentifier
	}
	value, err := uc.Store.OwnedCollection(ctx, userID, id)
	if ctx.Err() != nil {
		return domain.Collection{}, domain.ErrCollectionStore
	}
	return value, err
}
