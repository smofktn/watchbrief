// Package store adapts the sqlc queries to the small authentication boundary.
package store

//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate -f ../../sqlc.yaml

import (
 "context"
 "errors"

 "github.com/jackc/pgx/v5"
 "github.com/jackc/pgx/v5/pgxpool"
 "github.com/smofktn/watchbrief/backend/internal/db"
)

type User struct {
 ID string `json:"id"`
 LoginID string `json:"login_id"`
 PasswordHash string `json:"-"`
}

var ErrNotFound = errors.New("not found")

type Store struct { q *db.Queries }
func New(pool *pgxpool.Pool) *Store { return &Store{q: db.New(pool)} }
func (s *Store) UserByLogin(ctx context.Context, login string) (User, error) {
 row, err := s.q.UserByLogin(ctx, login)
 if errors.Is(err, pgx.ErrNoRows) { return User{}, ErrNotFound }
 return User{ID: row.ID, LoginID: row.LoginID, PasswordHash: row.PasswordHash}, err
}
func (s *Store) UserByToken(ctx context.Context, hash []byte) (User, error) {
 row, err := s.q.UserByToken(ctx, hash)
 if errors.Is(err, pgx.ErrNoRows) { return User{}, ErrNotFound }
 return User{ID: row.ID, LoginID: row.LoginID}, err
}
func (s *Store) CreateSession(ctx context.Context, id, userID string, hash []byte) error {
 return s.q.CreateSession(ctx, db.CreateSessionParams{ID: id, UserID: userID, TokenHash: hash})
}
func (s *Store) CreateUser(ctx context.Context, id, login, hash string) error {
 return s.q.CreateUser(ctx, db.CreateUserParams{ID: id, LoginID: login, PasswordHash: hash})
}
