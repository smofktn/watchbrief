package usecase

import (
	"context"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type LogoutStore interface {
	DeleteSessionByHash(context.Context, [32]byte) error
}

type Logout struct{ Store LogoutStore }

func (uc Logout) Execute(ctx context.Context, digest [32]byte) error {
	if ctx.Err() != nil || uc.Store == nil {
		return domain.ErrSessionStore
	}
	// An absent session is already invalidated; retrying must remain safe.
	if err := uc.Store.DeleteSessionByHash(ctx, digest); err != nil || ctx.Err() != nil {
		return domain.ErrSessionStore
	}
	return nil
}
