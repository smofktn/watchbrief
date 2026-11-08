package postgres

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres/generated"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

// A script checks operation order and bound values, without copying SQL text.
// Real SQL constraints, ownership and lock concurrency belong to DB tests.
type ioStep struct {
	name                  string
	args                  []any
	values                []any
	err                   error
	rows                  [][]any
	scanErr, iterationErr error
	affected              int
}
type scriptedDB struct {
	t        *testing.T
	steps    []ioStep
	rollback int
}

func (s *scriptedDB) next(name string, args []any) ioStep {
	s.t.Helper()
	if len(s.steps) == 0 {
		s.t.Fatalf("unexpected I/O %s", name)
	}
	step := s.steps[0]
	s.steps = s.steps[1:]
	if step.name != name || !reflect.DeepEqual(step.args, args) {
		s.t.Fatalf("unexpected I/O %s (expected %s); bound arguments match: %v", name, step.name, reflect.DeepEqual(step.args, args))
	}
	return step
}
func newScript(t *testing.T, steps ...ioStep) *scriptedDB {
	s := &scriptedDB{t: t, steps: steps}
	t.Cleanup(func() {
		if len(s.steps) > 0 {
			t.Errorf("%d expected operations not executed", len(s.steps))
		}
	})
	return s
}
func queryName(q string) string { return strings.Fields(q)[2] }
func (s *scriptedDB) Exec(_ context.Context, q string, args ...any) (pgconn.CommandTag, error) {
	step := s.next(queryName(q), args)
	return pgconn.NewCommandTag("UPDATE " + strconv.Itoa(step.affected)), step.err
}
func (s *scriptedDB) QueryRow(_ context.Context, q string, args ...any) pgx.Row {
	step := s.next(queryName(q), args)
	return fakeRow{step.values, step.err}
}
func (s *scriptedDB) Query(_ context.Context, q string, args ...any) (pgx.Rows, error) {
	step := s.next(queryName(q), args)
	rows := &fakeRows{values: step.rows, scanErr: step.scanErr, err: step.iterationErr}
	if step.err == nil {
		s.t.Cleanup(func() {
			if !rows.closed {
				s.t.Error("database rows not closed")
			}
		})
	}
	return rows, step.err
}
func (s *scriptedDB) BeginTx(_ context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	if opts.IsoLevel != pgx.ReadCommitted {
		s.t.Fatal("credential transaction must read committed state after lock")
	}
	step := s.next("begin", nil)
	return &fakeTx{db: s}, step.err
}

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("test scan arity mismatch")
	}
	for i, v := range r.values {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(v))
	}
	return nil
}

type fakeRows struct {
	pgx.Rows
	values       [][]any
	scanErr, err error
	index        int
	closed       bool
}

func (r *fakeRows) Next() bool {
	if r.index >= len(r.values) {
		return false
	}
	r.index++
	return true
}
func (r *fakeRows) Scan(dest ...any) error {
	return (fakeRow{r.values[r.index-1], r.scanErr}).Scan(dest...)
}
func (r *fakeRows) Err() error { return r.err }
func (r *fakeRows) Close()     { r.closed = true }

type fakeTx struct {
	pgx.Tx
	db *scriptedDB
}

func (t *fakeTx) Exec(c context.Context, q string, a ...any) (pgconn.CommandTag, error) {
	return t.db.Exec(c, q, a...)
}
func (t *fakeTx) QueryRow(c context.Context, q string, a ...any) pgx.Row {
	return t.db.QueryRow(c, q, a...)
}
func (t *fakeTx) Commit(context.Context) error { return t.db.next("commit", nil).err }
func (t *fakeTx) Rollback(ctx context.Context) error {
	if ctx.Err() != nil {
		t.db.t.Error("rollback must survive request cancellation")
	}
	t.db.rollback++
	return nil
}

var driverSecret = errors.New("fictional secret must not escape driver boundary")

func TestUserMappingAndErrorClassification(t *testing.T) {
	ctx := context.Background()
	digest := [32]byte{7}
	t.Run("user mapping", func(t *testing.T) {
		s := NewUserStore(newScript(t, ioStep{name: "UserByLoginID", args: []any{"fictional"}, values: []any{"user", "fictional", "phc"}}))
		got, err := s.ByLoginID(ctx, "fictional")
		if err != nil || got != (domain.User{ID: "user", LoginID: "fictional", PasswordHash: "phc"}) {
			t.Fatal("mapping failed")
		}
	})
	for _, tc := range []struct {
		name        string
		input, want error
	}{{"missing", pgx.ErrNoRows, domain.ErrUserNotFound}, {"driver", driverSecret, domain.ErrUserStore}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewUserStore(newScript(t, ioStep{name: "UserByLoginID", args: []any{"fictional"}, err: tc.input})).ByLoginID(ctx, "fictional")
			if err != tc.want {
				t.Fatalf("error not sanitized: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		name        string
		input, want error
	}{{"created", nil, nil}, {"duplicate", &pgconn.PgError{Code: "23505", ConstraintName: "users_login_id_key"}, domain.ErrDuplicateLoginID}, {"other unique", &pgconn.PgError{Code: "23505", ConstraintName: "users_pkey"}, domain.ErrUserStore}, {"driver", driverSecret, domain.ErrUserStore}} {
		t.Run(tc.name, func(t *testing.T) {
			err := NewUserStore(newScript(t, ioStep{name: "CreateUser", args: []any{"user", "fictional", "phc"}, err: tc.input})).Create(ctx, domain.User{ID: "user", LoginID: "fictional", PasswordHash: "phc"})
			if err != tc.want {
				t.Fatalf("unexpected classification: %v", err)
			}
		})
	}
	for _, method := range []string{"UserBySessionHash", "UserIDBySessionHash"} {
		for _, input := range []error{nil, pgx.ErrNoRows, driverSecret} {
			t.Run(method+"/"+errorName(input), func(t *testing.T) {
				s := NewUserStore(newScript(t, ioStep{name: method, args: []any{digest[:]}, values: []any{"identity"}, err: input}))
				var got string
				var err error
				if method == "UserBySessionHash" {
					got, err = s.LoginIDBySessionHash(ctx, digest)
				} else {
					got, err = s.UserIDBySessionHash(ctx, digest)
				}
				want := error(nil)
				if input == pgx.ErrNoRows {
					want = domain.ErrUnauthenticated
				} else if input != nil {
					want = domain.ErrSessionStore
				}
				if err != want || (want == nil && got != "identity") || (want != nil && got != "") {
					t.Fatal("incorrect authentication mapping")
				}
			})
		}
	}
	for _, input := range []error{nil, driverSecret} {
		t.Run("delete/"+errorName(input), func(t *testing.T) {
			err := NewUserStore(newScript(t, ioStep{name: "DeleteSessionByHash", args: []any{digest[:]}, err: input})).DeleteSessionByHash(ctx, digest)
			if (input == nil && err != nil) || (input != nil && err != domain.ErrSessionStore) {
				t.Fatal("incorrect deletion classification")
			}
		})
	}
}
func errorName(err error) string {
	if err == nil {
		return "success"
	}
	if err == pgx.ErrNoRows {
		return "missing"
	}
	return "failure"
}
func TestOwnedLists(t *testing.T) {
	stamp := pgtype.Timestamptz{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}
	for _, reports := range []bool{false, true} {
		name := "ListOwnedCollections"
		sentinel := domain.ErrCollectionStore
		row := []any{"id", "name", true, int32(3)}
		if reports {
			name = "ListOwnedReports"
			sentinel = domain.ErrReportStore
			row = []any{"id", "name", stamp, stamp}
		}
		for _, scenario := range []string{"empty", "populated", "query failure", "scan failure", "iteration failure"} {
			t.Run(name+"/"+scenario, func(t *testing.T) {
				step := ioStep{name: name, args: []any{"owner"}}
				switch scenario {
				case "populated":
					step.rows = [][]any{row}
				case "query failure":
					step.err = driverSecret
				case "scan failure":
					step.rows = [][]any{row}
					step.scanErr = driverSecret
				case "iteration failure":
					step.rows = [][]any{row}
					step.iterationErr = driverSecret
				}
				s := NewUserStore(newScript(t, step))
				var err error
				length := 0
				nonNil := false
				if reports {
					var got []domain.ReportSummary
					got, err = s.ListOwnedReports(context.Background(), "owner")
					length = len(got)
					nonNil = got != nil
					if length > 0 && (got[0].ID != "id" || got[0].CollectionName != "name" || got[0].ScheduledAt != stamp.Time || got[0].GeneratedAt != stamp.Time) {
						t.Fatal("report mapping")
					}
				} else {
					var got []domain.CollectionSummary
					got, err = s.ListOwnedCollections(context.Background(), "owner")
					length = len(got)
					nonNil = got != nil
					if length > 0 && got[0] != (domain.CollectionSummary{ID: "id", Name: "name", Enabled: true, DisplayOrder: 3}) {
						t.Fatal("collection mapping")
					}
				}
				if strings.Contains(scenario, "failure") {
					if err != sentinel || nonNil {
						t.Fatal("partial results or driver error escaped")
					}
				} else if err != nil || !nonNil || (scenario == "populated" && length != 1) || (scenario == "empty" && length != 0) {
					t.Fatal("list contract failed")
				}
			})
		}
	}
}
func TestOwnedDetails(t *testing.T) {
	ctx := context.Background()
	stamp := pgtype.Timestamptz{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}
	for _, reports := range []bool{false, true} {
		for _, scenario := range []string{"success", "missing", "driver failure", "invalid JSON"} {
			if reports && scenario == "invalid JSON" {
				continue
			}
			name := "OwnedCollectionDocument"
			want := error(nil)
			values := []any{[]byte(`{"id":"id","name":"name","targets":[],"schedule":null}`)}
			if reports {
				name = "OwnedReport"
				values = []any{"id", "name", stamp, stamp, "body", "summary", stamp, stamp}
			}
			t.Run(name+"/"+scenario, func(t *testing.T) {
				step := ioStep{name: name, args: []any{"id", "owner"}, values: values}
				switch scenario {
				case "missing":
					step.err = pgx.ErrNoRows
					want = domain.ErrResourceNotFound
				case "driver failure":
					step.err = driverSecret
					want = domain.ErrCollectionStore
					if reports {
						want = domain.ErrReportStore
					}
				case "invalid JSON":
					step.values = []any{[]byte(`{broken`)}
					want = domain.ErrCollectionStore
				}
				s := NewUserStore(newScript(t, step))
				var err error
				if reports {
					var got domain.Report
					got, err = s.OwnedReport(ctx, "owner", "id")
					if err == nil && (got.ID != "id" || got.CollectionName != "name" || got.BodyMarkdown != "body" || got.SummaryMarkdown != "summary" || got.ScheduledAt != stamp.Time || got.GeneratedAt != stamp.Time || got.UpdatesStartedAt != stamp.Time || got.UpdatesEndedAt != stamp.Time) {
						t.Fatal("report detail mapping")
					}
				} else {
					var got domain.Collection
					got, err = s.OwnedCollection(ctx, "owner", "id")
					if err == nil && (got.ID != "id" || got.Name != "name" || got.Targets == nil || got.Schedule != nil) {
						t.Fatal("collection document mapping")
					}
				}
				if err != want {
					t.Fatalf("unexpected detail classification: %v", err)
				}
			})
		}
	}
}
func TestCredentialTransactionBoundaries(t *testing.T) {
	digest := [32]byte{9}
	session := domain.Session{ID: "session", UserID: "user", TokenHash: digest}
	for _, reset := range []bool{false, true} {
		for _, scenario := range []string{"success", "begin failure", "lock failure", "lookup failure", "changed password", "write failure", "delete failure", "commit failure"} {
			if !reset && scenario == "delete failure" {
				continue
			}
			name := "login"
			if reset {
				name = "reset"
			}
			t.Run(name+"/"+scenario, func(t *testing.T) {
				var steps []ioStep
				want := error(nil)
				if reset {
					steps = append(steps, ioStep{name: "UserIDByLoginID", args: []any{"fictional"}, values: []any{"user"}})
				}
				begin := ioStep{name: "begin"}
				if scenario == "begin failure" {
					begin.err = driverSecret
				}
				steps = append(steps, begin)
				if scenario != "begin failure" {
					lock := ioStep{name: "LockUserCredentials", args: []any{"user"}}
					if scenario == "lock failure" {
						lock.err = driverSecret
					}
					steps = append(steps, lock)
					if scenario != "lock failure" {
						if reset {
							update := ioStep{name: "ResetUserPassword", args: []any{"user", "new-phc"}, affected: 1}
							switch scenario {
							case "lookup failure", "write failure":
								update.err = driverSecret
							case "changed password":
								update.affected = 0
							}
							steps = append(steps, update)
							if update.err == nil && update.affected == 1 {
								del := ioStep{name: "DeleteUserSessions", args: []any{"user"}}
								if scenario == "delete failure" {
									del.err = driverSecret
								}
								steps = append(steps, del)
							}
						} else {
							read := ioStep{name: "UserPasswordHash", args: []any{"user"}, values: []any{"verified-phc"}}
							switch scenario {
							case "lookup failure":
								read.err = driverSecret
							case "changed password":
								read.values = []any{"reset-phc"}
							}
							steps = append(steps, read)
							if scenario != "lookup failure" && scenario != "changed password" {
								save := ioStep{name: "CreateSession", args: []any{"session", "user", digest[:]}}
								if scenario == "write failure" {
									save.err = driverSecret
								}
								steps = append(steps, save)
							}
						}
						if scenario == "success" || scenario == "commit failure" {
							commit := ioStep{name: "commit"}
							if scenario == "commit failure" {
								commit.err = driverSecret
							}
							steps = append(steps, commit)
						}
					}
				}
				if scenario != "success" {
					want = domain.ErrUserStore
					if !reset && (scenario == "lookup failure" || scenario == "write failure") {
						want = domain.ErrSessionStore
					}
					if scenario == "changed password" {
						want = domain.ErrPasswordChanged
						if reset {
							want = domain.ErrUserNotFound
						}
					}
				}
				db := newScript(t, steps...)
				s := NewUserStore(db)
				var err error
				if reset {
					err = s.ResetPassword(context.Background(), "fictional", "new-phc")
				} else {
					err = s.CreateSession(context.Background(), session, "verified-phc")
				}
				if err != want {
					t.Fatalf("classification = %v; want %v", err, want)
				}
				expectedRollback := 1
				if scenario == "begin failure" {
					expectedRollback = 0
				}
				if db.rollback != expectedRollback {
					t.Fatal("missing transaction cleanup")
				}
			})
		}
	}
	for _, input := range []error{pgx.ErrNoRows, driverSecret} {
		t.Run("reset missing/"+errorName(input), func(t *testing.T) {
			err := NewUserStore(newScript(t, ioStep{name: "UserIDByLoginID", args: []any{"fictional"}, err: input})).ResetPassword(context.Background(), "fictional", "new-phc")
			want := domain.ErrUserStore
			if input == pgx.ErrNoRows {
				want = domain.ErrUserNotFound
			}
			if err != want {
				t.Fatal("lookup classification")
			}
		})
	}
}

type queryOnlyDB struct{ generated.DBTX }

func TestCredentialWriterRequiresTransactionSupport(t *testing.T) {
	db := newScript(t)
	s := NewUserStore(queryOnlyDB{db})
	if err := s.CreateSession(context.Background(), domain.Session{}, "phc"); err != domain.ErrUserStore {
		t.Fatal("nontransactional credential write accepted")
	}
}
func TestSessionRejectsUserRemovedAfterVerification(t *testing.T) {
	db := newScript(t, ioStep{name: "begin"}, ioStep{name: "LockUserCredentials", args: []any{"user"}}, ioStep{name: "UserPasswordHash", args: []any{"user"}, err: pgx.ErrNoRows})
	if err := NewUserStore(db).CreateSession(context.Background(), domain.Session{UserID: "user"}, "phc"); err != domain.ErrPasswordChanged || db.rollback != 1 {
		t.Fatal("removed user created session or skipped rollback")
	}
}
