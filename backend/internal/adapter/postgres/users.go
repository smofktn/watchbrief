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
	db      generated.DBTX
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

// Each credential writer uses this same transaction lock. Hashing and password
// input happen before beginning the transaction. ReadCommitted is intentional:
// the query AFTER a waiting lock must see a reset that just committed.
func (s *UserStore) credentialTransaction(ctx context.Context, userID string, work func(*generated.Queries) error) error {
	beginner, ok := s.db.(interface {
		BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
	})
	if !ok {
		return domain.ErrUserStore
	}
	tx, err := beginner.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.ErrUserStore
	}
	defer tx.Rollback(context.Background())
	q := generated.New(tx)
	if err := q.LockUserCredentials(ctx, userID); err != nil {
		return domain.ErrUserStore
	}
	if err := work(q); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ErrUserStore
	}
	return nil
}

func (s *UserStore) CreateSession(ctx context.Context, session domain.Session, expectedHash string) error {
	return s.credentialTransaction(ctx, session.UserID, func(q *generated.Queries) error {
		current, err := q.UserPasswordHash(ctx, session.UserID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && current != expectedHash) {
			return domain.ErrPasswordChanged
		}
		if err != nil {
			return domain.ErrSessionStore
		}
		if err := q.CreateSession(ctx, generated.CreateSessionParams{ID: session.ID, UserID: session.UserID, TokenHash: session.TokenHash[:]}); err != nil {
			return domain.ErrSessionStore
		}
		return nil
	})
}

func (s *UserStore) ResetPassword(ctx context.Context, loginID, hash string) error {
	// The resetter can read identifiers, but deliberately cannot read old hashes.
	userID, err := s.queries.UserIDByLoginID(ctx, loginID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrUserNotFound
		}
		return domain.ErrUserStore
	}
	return s.credentialTransaction(ctx, userID, func(q *generated.Queries) error {
		count, err := q.ResetUserPassword(ctx, generated.ResetUserPasswordParams{ID: userID, PasswordHash: hash})
		if err != nil {
			return domain.ErrUserStore
		}
		if count != 1 {
			return domain.ErrUserNotFound
		}
		if err := q.DeleteUserSessions(ctx, userID); err != nil {
			return domain.ErrUserStore
		}
		return nil
	})
}

func NewUserStore(db generated.DBTX) *UserStore {
	return &UserStore{queries: generated.New(db), db: db}
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

func (s *UserStore) DeleteSessionByHash(ctx context.Context, digest [32]byte) error {
	if err := s.queries.DeleteSessionByHash(ctx, digest[:]); err != nil {
		return domain.ErrSessionStore
	}
	return nil
}
