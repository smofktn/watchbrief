package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres/generated"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type UserStore struct {
	queries *generated.Queries
}

func (s *UserStore) ByLoginID(ctx context.Context, loginID string) (domain.User, error) {
	row, err := s.queries.UserByLoginID(ctx, loginID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, domain.ErrUserStore
	}
	return domain.User{ID: row.ID, LoginID: row.LoginID, PasswordHash: row.PasswordHash}, nil
}

func (s *UserStore) CreateSession(ctx context.Context, session domain.Session) error {
	err := s.queries.CreateSession(ctx, generated.CreateSessionParams{ID: session.ID, UserID: session.UserID, TokenHash: session.TokenHash[:]})
	if err != nil {
		return domain.ErrSessionStore
	}
	return nil
}

func NewUserStore(db generated.DBTX) *UserStore {
	return &UserStore{queries: generated.New(db)}
}

func (s *UserStore) Create(ctx context.Context, user domain.User) error {
	err := s.queries.CreateUser(ctx, generated.CreateUserParams{
		ID: user.ID, LoginID: user.LoginID, PasswordHash: user.PasswordHash,
	})
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_login_id_key" {
		return domain.ErrDuplicateLoginID
	}
	// Driver errors may contain query parameters and password hashes.
	return domain.ErrUserStore
}

func (s *UserStore) LoginIDBySessionHash(ctx context.Context, digest [32]byte) (string, error) {
	loginID, err := s.queries.UserBySessionHash(ctx, digest[:])
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrUnauthenticated
	}
	if err != nil {
		return "", domain.ErrSessionStore
	}
	return loginID, nil
}
