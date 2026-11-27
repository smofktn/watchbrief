package usecase

import (
	"context"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type HomeStore interface {
	UserIDBySessionHash(context.Context, [32]byte) (string, error)
	ListHomeCollections(context.Context, string) ([]domain.HomeCollection, error)
}

type Home struct{ Store HomeStore }

func (uc Home) List(ctx context.Context, digest [32]byte, rawQuery string) (domain.Home, error) {
	if ctx.Err() != nil || uc.Store == nil {
		return domain.Home{}, domain.ErrHomeStore
	}
	userID, err := uc.Store.UserIDBySessionHash(ctx, digest)
	if err != nil {
		return domain.Home{}, err
	}
	if rawQuery != "" {
		return domain.Home{}, domain.ErrHomeRequest
	}
	collections, err := uc.Store.ListHomeCollections(ctx, userID)
	if ctx.Err() != nil || err != nil {
		return domain.Home{}, domain.ErrHomeStore
	}
	if collections == nil {
		collections = []domain.HomeCollection{}
	}
	return domain.Home{Collections: collections}, nil
}
