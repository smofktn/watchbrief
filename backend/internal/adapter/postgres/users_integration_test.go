//go:build integration

package postgres

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func TestUserStoreWithInsertOnlyRole(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	secret := filepath.Join("..", "..", "..", "..", ".local", "watchbrief-user-creator-password")
	conn, err := OpenUserCreator(ctx, secret)
	if err != nil {
		t.Fatal("local creator connection failed; prepare the local database first")
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal("cannot begin fixture transaction")
	}
	defer tx.Rollback(context.Background())
	store := NewUserStore(tx)
	user := domain.User{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAZ", LoginID: "sqlc_verification_user", PasswordHash: "synthetic-verification-only"}
	if err := store.Create(ctx, user); err != nil {
		t.Fatal("sqlc INSERT with insert-only role failed")
	}
	user.ID = "01ARZ3NDEKTSV4RRFFQ69G5FB0"
	if err := store.Create(ctx, user); !errors.Is(err, domain.ErrDuplicateLoginID) {
		t.Fatal("duplicate login ID was not classified safely")
	}
}

func TestUserStoreDoesNotExposeDriverErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	secret := filepath.Join("..", "..", "..", "..", ".local", "watchbrief-api-password")
	conn, err := openLocal(ctx, "watchbrief_api", secret)
	if err != nil {
		t.Fatal("local API connection failed")
	}
	defer conn.Close(context.Background())
	err = NewUserStore(conn).Create(ctx, domain.User{ID: "01ARZ3NDEKTSV4RRFFQ69G5FB0", LoginID: "sqlc_forbidden_user", PasswordHash: "synthetic-verification-only"})
	if !errors.Is(err, domain.ErrUserStore) {
		t.Fatal("API INSERT was not rejected with a sanitized storage error")
	}
}
