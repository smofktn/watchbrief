package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type HomeStore interface {
	UserIDBySessionHash(context.Context, [32]byte) (string, error)
	ListHomeCollections(context.Context, string, time.Time, time.Time) ([]domain.HomeCollection, error)
}

type Home struct {
	Store HomeStore
	Now   func() time.Time
}

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
	now := time.Now
	if uc.Now != nil {
		now = uc.Now
	}
	current := now()
	jst := time.FixedZone("JST", 9*60*60)
	localNow := current.In(jst)
	year, month, day := localNow.Date()
	start := time.Date(year, month, day, 0, 0, 0, 0, jst)
	end := start.AddDate(0, 0, 1)
	asOfDate := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	collections, err := uc.Store.ListHomeCollections(ctx, userID, start.UTC(), end.UTC())
	if ctx.Err() != nil || err != nil {
		return domain.Home{}, domain.ErrHomeStore
	}
	if collections == nil {
		collections = []domain.HomeCollection{}
	}
	return domain.Home{AsOfDate: asOfDate, Collections: collections}, nil
}
