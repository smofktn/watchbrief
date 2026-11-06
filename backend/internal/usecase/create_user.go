package usecase

import (
	"context"
	"crypto/subtle"
	"errors"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

var (
	ErrConfirmation = errors.New("password confirmation does not match")
	ErrCreateUser   = errors.New("cannot create user")
)

type UserWriter interface {
	Create(context.Context, domain.User) error
}

type PasswordHasher interface {
	Hash(context.Context, string) (string, error)
}

type CreateUser struct {
	Store  UserWriter
	Hasher PasswordHasher
	NewID  func() (string, error)
}

func (uc CreateUser) Execute(ctx context.Context, loginID, password, confirmation string) error {
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
	if uc.Store == nil || uc.Hasher == nil || uc.NewID == nil {
		return ErrCreateUser
	}
	hash, err := uc.Hasher.Hash(ctx, password)
	if err != nil {
		return ErrCreateUser
	}
	userID, err := uc.NewID()
	if err != nil {
		return ErrCreateUser
	}
	return uc.Store.Create(ctx, domain.User{ID: userID, LoginID: id, PasswordHash: hash})
}
