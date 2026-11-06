package usecase

import (
	"context"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type CurrentUserStore interface {
	LoginIDBySessionHash(context.Context, [32]byte) (string, error)
}

type CurrentUser struct{ Store CurrentUserStore }

func (uc CurrentUser) Execute(ctx context.Context, digest [32]byte) (string, error) {
	if ctx.Err() != nil || uc.Store == nil {
		return "", domain.ErrSessionStore
	}
	loginID, err := uc.Store.LoginIDBySessionHash(ctx, digest)
	if ctx.Err() != nil {
		return "", domain.ErrSessionStore
	}
	return loginID, err
}
