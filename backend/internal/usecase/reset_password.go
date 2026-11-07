package usecase

import (
	"context"
	"crypto/subtle"
	"errors"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

var ErrResetPassword = errors.New("cannot reset password")

type PasswordResetter interface {
	ResetPassword(context.Context, string, string) error
}
type ResetPassword struct {
	Store  PasswordResetter
	Hasher PasswordHasher
}

func (uc ResetPassword) Execute(ctx context.Context, loginID, password, confirmation string) error {
	id, err := domain.NormalizeLoginID(loginID)
	if err != nil {
		return err
	}
	password, err = domain.NormalizePassword(password)
	if err != nil {
		return err
	}
	confirmation, err = domain.NormalizePassword(confirmation)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(confirmation)) != 1 {
		return ErrConfirmation
	}
	if ctx.Err() != nil || uc.Store == nil || uc.Hasher == nil {
		return ErrResetPassword
	}
	hash, err := uc.Hasher.Hash(ctx, password)
	if err != nil {
		return ErrResetPassword
	}
	return uc.Store.ResetPassword(ctx, id, hash)
}
