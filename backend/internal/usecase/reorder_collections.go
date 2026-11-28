package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type CollectionOrderStore interface {
	UserIDBySessionHash(context.Context, [32]byte) (string, error)
	ReorderOwnedCollections(context.Context, string, string, [32]byte, domain.CollectionOrderInput) error
}

type ReorderCollections struct{ Store CollectionOrderStore }

func (uc ReorderCollections) Reorder(ctx context.Context, digest [32]byte, key string, input domain.CollectionOrderInput) error {
	if ctx.Err() != nil || uc.Store == nil {
		return domain.ErrCollectionStore
	}
	owner, err := uc.Store.UserIDBySessionHash(ctx, digest)
	if err != nil {
		return err
	}
	if !domain.ValidCreationKey(key) || input.Validate() != nil {
		return domain.ErrCollectionOrderInput
	}
	body, err := json.Marshal(input)
	if err != nil {
		return domain.ErrCollectionStore
	}
	return uc.Store.ReorderOwnedCollections(ctx, owner, key, sha256.Sum256(body), input)
}
