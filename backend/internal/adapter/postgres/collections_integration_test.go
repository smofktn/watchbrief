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

func TestCollectionStoreOwnerAndStableOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	secret := filepath.Join("..", "..", "..", "..", ".local", "watchbrief-migrator-password")
	conn, err := openLocal(ctx, "watchbrief_migrator", secret)
	if err != nil {
		t.Fatal("local migration connection failed")
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal("fixture transaction failed")
	}
	defer tx.Rollback(context.Background())
	const owner = "00000000000000000000000010"
	const other = "00000000000000000000000011"
	_, err = tx.Exec(ctx, `INSERT INTO app.users(id,login_id,password_hash) VALUES ($1,'collection_store_owner','synthetic'),($2,'collection_store_other','synthetic')`, owner, other)
	if err != nil {
		t.Fatal("fictional User insert failed")
	}
	_, err = tx.Exec(ctx, `INSERT INTO app.collections(id,user_id,name,display_order) VALUES
	('00000000000000000000000020',$1,'second',1),
	('00000000000000000000000021',$1,'third',1),
	('00000000000000000000000022',$1,'first',0),
	('00000000000000000000000023',$2,'other',0)`, owner, other)
	if err != nil {
		t.Fatal("fictional Collection insert failed")
	}
	store := NewUserStore(tx)
	rows, err := store.ListOwnedCollections(ctx, owner)
	if err != nil || len(rows) != 3 || rows[0].Name != "first" || rows[1].Name != "second" || rows[2].Name != "third" {
		t.Fatal("owner scope or stable order incorrect")
	}
	value, err := store.OwnedCollection(ctx, owner, "00000000000000000000000022")
	if err != nil || value.Schedule != nil || value.Targets == nil || len(value.Targets) != 0 {
		t.Fatal("optional Schedule/empty targets not represented correctly")
	}
	_, err = store.OwnedCollection(ctx, owner, "00000000000000000000000023")
	if !errors.Is(err, domain.ErrResourceNotFound) {
		t.Fatal("other owner detail exposed")
	}
}
