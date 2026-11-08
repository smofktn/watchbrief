package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type CollectionCreationStore interface {
	UserIDBySessionHash(context.Context, [32]byte) (string, error)
	CreateOwnedCollection(context.Context, string, string, [32]byte, string, domain.NewCollection) (string, bool, error)
}

type CreateCollection struct {
	Store CollectionCreationStore
	NewID func() (string, error)
}

func (uc CreateCollection) Execute(ctx context.Context, digest [32]byte, key string, input domain.NewCollection) (string, bool, error) {
	if ctx.Err() != nil || uc.Store == nil || uc.NewID == nil {
		return "", false, domain.ErrCollectionStore
	}
	user, err := uc.Store.UserIDBySessionHash(ctx, digest)
	if err != nil {
		return "", false, err
	}
	if !domain.ValidCreationKey(key) {
		return "", false, domain.ErrInvalidIdentifier
	}
	if err := input.Validate(); err != nil {
		return "", false, err
	}
	body, err := json.Marshal(input)
	if err != nil {
		return "", false, domain.ErrCollectionStore
	}
	id, err := uc.NewID()
	if err != nil || !domain.ValidIdentifier(id) {
		return "", false, domain.ErrCollectionStore
	}
	return uc.Store.CreateOwnedCollection(ctx, user, key, sha256.Sum256(body), id, input)
}
