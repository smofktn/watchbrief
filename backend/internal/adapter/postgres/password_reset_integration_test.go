//go:build integration

package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/smofktn/watchbrief/backend/internal/domain"
	"github.com/smofktn/watchbrief/backend/internal/security/identifier"
	"github.com/smofktn/watchbrief/backend/internal/security/password"
	"github.com/smofktn/watchbrief/backend/internal/security/token"
	"github.com/smofktn/watchbrief/backend/internal/usecase"
)

type credentialTestDB struct {
	*pgx.Conn
	locked     chan struct{}
	release    chan struct{}
	failDelete bool
}
type credentialTestTx struct {
	pgx.Tx
	owner *credentialTestDB
}

func (d *credentialTestDB) BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	tx, err := d.Conn.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &credentialTestTx{tx, d}, nil
}
func (tx *credentialTestTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if tx.owner.failDelete && strings.Contains(sql, "-- name: DeleteUserSessions") {
		// A real PostgreSQL statement error AFTER the real UPDATE; no global grants or triggers change.
		return tx.Tx.Exec(ctx, "DELETE FROM app.auth_sessions WHERE user_id=$1 AND (1/0)=0", args...)
	}
	tag, err := tx.Tx.Exec(ctx, sql, args...)
	if err == nil && strings.Contains(sql, "pg_advisory_xact_lock") && tx.owner.locked != nil {
		close(tx.owner.locked)
		select {
		case <-tx.owner.release:
		case <-ctx.Done():
			return tag, ctx.Err()
		}
	}
	return tag, err
}

func TestPasswordResetTransactions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	secret := func(name string) string { return filepath.Join("..", "..", "..", "..", ".local", name) }
	connect := func(role, file string) *pgx.Conn {
		c, e := openLocal(ctx, role, secret(file))
		if e != nil {
			t.Fatal("verification connection failed")
		}
		t.Cleanup(func() { c.Close(context.Background()) })
		return c
	}
	creator := connect("watchbrief_user_creator", "watchbrief-user-creator-password")
	api := connect("watchbrief_api", "watchbrief-api-password")
	reset := connect("watchbrief_password_resetter", "watchbrief-password-resetter-password")
	id, e := identifier.NewULID()
	if e != nil {
		t.Fatal(e)
	}
	loginID := "verify_reset_race_" + strings.ToLower(id)
	hasher := password.NewHasher()
	hashes := map[string]string{}
	passwords := map[string]string{}
	for _, name := range []string{"initial", "next", "final", "penultimate", "winner", "must-rollback", "after-cancel", "cancelled"} {
		raw, _, err := token.New()
		if err != nil {
			t.Fatal("fixture secret generation failed")
		}
		passwords[name] = base64.RawURLEncoding.EncodeToString(raw[:])
		encoded, err := hasher.Hash(ctx, passwords[name])
		if err != nil {
			t.Fatal("fixture hashing failed")
		}
		hashes[name] = encoded
	}
	login := func(store usecase.LoginStore, name string) error {
		_, err := (usecase.Login{Store: store, Verifier: hasher, DummyHash: hashes["initial"], NewID: identifier.NewULID, NewToken: token.New}).Execute(ctx, loginID, passwords[name])
		return err
	}
	if e := NewUserStore(creator).Create(ctx, domain.User{ID: id, LoginID: loginID, PasswordHash: hashes["initial"]}); e != nil {
		t.Fatal(e)
	}
	// Dedicated fixture with real Argon2id hashes is retained; no existing user's password is changed.
	count := func() int {
		var n int
		if e := api.QueryRow(ctx, "SELECT count(*) FROM app.auth_sessions WHERE user_id=$1", id).Scan(&n); e != nil {
			t.Fatal("count failed")
		}
		return n
	}
	hash := func() string {
		u, e := NewUserStore(api).ByLoginID(ctx, loginID)
		if e != nil {
			t.Fatal(e)
		}
		return u.PasswordHash
	}
	session := func() domain.Session {
		sid, e := identifier.NewULID()
		if e != nil {
			t.Fatal(e)
		}
		return domain.Session{ID: sid, UserID: id, TokenHash: sha256.Sum256([]byte(sid))}
	}
	wait := func(ch <-chan struct{}) {
		select {
		case <-ch:
		case <-ctx.Done():
			t.Fatal("barrier timed out")
		}
	}
	result := func(ch <-chan error) error {
		select {
		case e := <-ch:
			return e
		case <-ctx.Done():
			t.Fatal("operation timed out")
			return ctx.Err()
		}
	}

	// Login has the lock: reset must wait, then delete the just-committed session.
	loginDB := &credentialTestDB{Conn: api, locked: make(chan struct{}), release: make(chan struct{})}
	loginDone := make(chan error, 1)
	go func() { loginDone <- login(NewUserStore(loginDB), "initial") }()
	wait(loginDB.locked)
	resetDone := make(chan error, 1)
	go func() { resetDone <- NewUserStore(reset).ResetPassword(ctx, loginID, hashes["next"]) }()
	assertWaiting(t, ctx, reset, resetDone)
	close(loginDB.release)
	if e := result(loginDone); e != nil {
		t.Fatal(e)
	}
	if e := result(resetDone); e != nil {
		t.Fatal(e)
	}
	if count() != 0 || hash() != hashes["next"] {
		t.Fatal("login-before-reset escaped invalidation")
	}

	// Reset has the lock: an old already-verified PHC cannot create a session after it commits.
	resetDB := &credentialTestDB{Conn: reset, locked: make(chan struct{}), release: make(chan struct{})}
	go func() { resetDone <- NewUserStore(resetDB).ResetPassword(ctx, loginID, hashes["final"]) }()
	wait(resetDB.locked)
	go func() { loginDone <- login(NewUserStore(api), "next") }()
	assertWaiting(t, ctx, api, loginDone)
	close(resetDB.release)
	if e := result(resetDone); e != nil {
		t.Fatal(e)
	}
	if e := result(loginDone); !errors.Is(e, usecase.ErrInvalidCredentials) {
		t.Fatal("stale PHC accepted")
	}
	if count() != 0 {
		t.Fatal("stale login left session")
	}

	// Two resetters serialize; the last committed reset wins, all old sessions are deleted.
	reset2 := connect("watchbrief_password_resetter", "watchbrief-password-resetter-password")
	if e := NewUserStore(api).CreateSession(ctx, session(), hashes["final"]); e != nil {
		t.Fatal(e)
	}
	resetDB = &credentialTestDB{Conn: reset, locked: make(chan struct{}), release: make(chan struct{})}
	go func() { resetDone <- NewUserStore(resetDB).ResetPassword(ctx, loginID, hashes["penultimate"]) }()
	wait(resetDB.locked)
	secondDone := make(chan error, 1)
	go func() { secondDone <- NewUserStore(reset2).ResetPassword(ctx, loginID, hashes["winner"]) }()
	assertWaiting(t, ctx, reset2, secondDone)
	close(resetDB.release)
	if e := result(resetDone); e != nil {
		t.Fatal(e)
	}
	if e := result(secondDone); e != nil {
		t.Fatal(e)
	}
	matched, verifyErr := hasher.Verify(ctx, passwords["winner"], hash())
	if verifyErr != nil || !matched {
		t.Fatal("winning reset password did not verify")
	}
	if hash() != hashes["winner"] || count() != 0 {
		t.Fatal("concurrent resets inconsistent")
	}

	if e := NewUserStore(api).CreateSession(ctx, session(), hashes["winner"]); e != nil {
		t.Fatal(e)
	}
	var beforeUpdateTime time.Time
	if err := api.QueryRow(ctx, "SELECT updated_at FROM app.users WHERE id=$1", id).Scan(&beforeUpdateTime); err != nil {
		t.Fatal("timestamp read failed")
	}
	if e := NewUserStore(&credentialTestDB{Conn: reset, failDelete: true}).ResetPassword(ctx, loginID, hashes["must-rollback"]); !errors.Is(e, domain.ErrUserStore) {
		t.Fatal("delete failure not sanitized")
	}
	var afterUpdateTime time.Time
	if err := api.QueryRow(ctx, "SELECT updated_at FROM app.users WHERE id=$1", id).Scan(&afterUpdateTime); err != nil {
		t.Fatal("timestamp read failed")
	}
	if !beforeUpdateTime.Equal(afterUpdateTime) || hash() != hashes["winner"] || count() != 1 {
		t.Fatal("partial reset committed")
	}

	// Wait cancellation must release its transaction without touching password/session.
	resetDB = &credentialTestDB{Conn: reset, locked: make(chan struct{}), release: make(chan struct{})}
	go func() { resetDone <- NewUserStore(resetDB).ResetPassword(ctx, loginID, hashes["after-cancel"]) }()
	wait(resetDB.locked)
	short, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	err := NewUserStore(reset2).ResetPassword(short, loginID, hashes["cancelled"])
	stop()
	if err == nil {
		t.Fatal("cancelled waiter succeeded")
	}
	close(resetDB.release)
	if e := result(resetDone); e != nil {
		t.Fatal(e)
	}
	if hash() != hashes["after-cancel"] || count() != 0 {
		t.Fatal("cancellation changed state")
	}
}

// Observe the actual blocked backend from a SEPARATE connection; no sleeps are
// used as evidence of serialization. Only lock state is observed.
func assertWaiting(t *testing.T, ctx context.Context, conn *pgx.Conn, done <-chan error) {
	t.Helper()
	observer, err := openLocal(ctx, "watchbrief_api", filepath.Join("..", "..", "..", "..", ".local", "watchbrief-api-password"))
	if err != nil {
		t.Fatal("observer connection failed")
	}
	defer observer.Close(context.Background())
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		// pg_locks is visible across roles without privileged query-text access.
		if err := observer.QueryRow(ctx, "SELECT EXISTS(SELECT FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND NOT granted)", conn.PgConn().PID()).Scan(&waiting); err != nil {
			t.Fatal("lock observation failed")
		}
		if waiting {
			return
		}
		select {
		case <-done:
			t.Fatal("operation did not wait for credential lock")
		case <-deadline.C:
			t.Fatal("advisory lock wait not observed")
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("lock observation cancelled")
		}
	}
}
