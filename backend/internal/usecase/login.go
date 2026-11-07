package usecase

import (
	"context"
	"errors"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrLogin              = errors.New("login operation failed")
)

type LoginStore interface {
	ByLoginID(context.Context, string) (domain.User, error)
	CreateSession(context.Context, domain.Session, string) error
}

type PasswordVerifier interface {
	Verify(context.Context, string, string) (bool, error)
}

type Login struct {
	Store     LoginStore
	Verifier  PasswordVerifier
	DummyHash string
	NewID     func() (string, error)
	NewToken  func() ([32]byte, [32]byte, error)
}

func (uc Login) Execute(ctx context.Context, loginID, password string) ([32]byte, error) {
	var empty [32]byte
	if ctx.Err() != nil {
		return empty, ErrLogin
	}
	id, err := domain.ValidateLoginInput(loginID, password)
	if err != nil {
		return empty, err
	}
	if uc.Store == nil || uc.Verifier == nil || uc.NewID == nil || uc.NewToken == nil || uc.DummyHash == "" {
		return empty, ErrLogin
	}
	user, err := uc.Store.ByLoginID(ctx, id)
	missing := errors.Is(err, domain.ErrUserNotFound)
	if err != nil && !missing {
		return empty, ErrLogin
	}
	encoded := user.PasswordHash
	if missing {
		encoded = uc.DummyHash
	}
	matched, err := uc.Verifier.Verify(ctx, password, encoded)
	if err != nil {
		return empty, ErrLogin
	}
	if missing || !matched {
		return empty, ErrInvalidCredentials
	}
	raw, digest, err := uc.NewToken()
	if err != nil {
		return empty, ErrLogin
	}
	sessionID, err := uc.NewID()
	if err != nil {
		return empty, ErrLogin
	}
	if err := uc.Store.CreateSession(ctx, domain.Session{ID: sessionID, UserID: user.ID, TokenHash: digest}, user.PasswordHash); err != nil {
		if errors.Is(err, domain.ErrPasswordChanged) {
			return empty, ErrInvalidCredentials
		}
		return empty, ErrLogin
	}
	if ctx.Err() != nil {
		return empty, ErrLogin
	}
	return raw, nil
}
