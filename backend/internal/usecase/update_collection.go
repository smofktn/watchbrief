package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type CollectionUpdateStore interface {
	UserIDBySessionHash(context.Context, [32]byte) (string, error)
	UpdateOwnedCollection(context.Context, string, string, string, [32]byte, domain.CollectionUpdate) (domain.CollectionUpdateResult, error)
}

type UpdateCollection struct{ Store CollectionUpdateStore }

func (uc UpdateCollection) Update(ctx context.Context, digest [32]byte, id, key string, input domain.CollectionUpdate) (domain.CollectionUpdateResult, error) {
	var empty domain.CollectionUpdateResult
	if ctx.Err() != nil || uc.Store == nil {
		return empty, domain.ErrCollectionStore
	}
	owner, err := uc.Store.UserIDBySessionHash(ctx, digest)
	if err != nil {
		return empty, err
	}
	if !domain.ValidIdentifier(id) || !domain.ValidCreationKey(key) {
		return empty, domain.ErrInvalidIdentifier
	}
	if err := input.Validate(); err != nil {
		return empty, err
	}
	body, err := json.Marshal(struct {
		ID string `json:"id"`
		domain.CollectionUpdate
	}{id, input})
	if err != nil {
		return empty, domain.ErrCollectionStore
	}
	return uc.Store.UpdateOwnedCollection(ctx, owner, id, key, sha256.Sum256(body), input)
}
