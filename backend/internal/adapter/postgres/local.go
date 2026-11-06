package postgres

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

var ErrConnection = errors.New("cannot connect to the local database with the required role")

// LocalUserCreator opens its connection only when the use case is ready to save.
type LocalUserCreator struct{ passwordFile string }

func NewLocalUserCreator(passwordFile string) *LocalUserCreator {
	return &LocalUserCreator{passwordFile: passwordFile}
}

func (s *LocalUserCreator) Create(ctx context.Context, user domain.User) error {
	conn, err := OpenUserCreator(ctx, s.passwordFile)
	if err != nil {
		return domain.ErrUserStore
	}
	defer conn.Close(context.Background())
	return NewUserStore(conn).Create(ctx, user)
}

// OpenUserCreator connects only to the dedicated local DB with the insert-only role.
func OpenUserCreator(ctx context.Context, passwordFile string) (*pgx.Conn, error) {
	return openLocal(ctx, "watchbrief_user_creator", passwordFile)
}

func openLocal(ctx context.Context, role, passwordFile string) (*pgx.Conn, error) {
	password, err := readLocalPassword(passwordFile)
	if err != nil {
		return nil, err
	}
	cfg, err := pgx.ParseConfig("host=127.0.0.1 port=5432 dbname=watchbrief user=" + role + " sslmode=disable connect_timeout=5")
	if err != nil {
		return nil, ErrConnection
	}
	cfg.Password = password
	cfg.RuntimeParams = map[string]string{"search_path": "app,pg_catalog", "timezone": "UTC"}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, ErrConnection
	}
	return conn, nil
}

func OpenAPI(ctx context.Context, passwordFile string) (*pgxpool.Pool, error) {
	password, err := readLocalPassword(passwordFile)
	if err != nil {
		return nil, err
	}
	cfg, err := pgxpool.ParseConfig("host=127.0.0.1 port=5432 dbname=watchbrief user=watchbrief_api sslmode=disable connect_timeout=5")
	if err != nil {
		return nil, ErrConnection
	}
	cfg.ConnConfig.Password = password
	cfg.ConnConfig.RuntimeParams = map[string]string{"search_path": "app,pg_catalog", "timezone": "UTC"}
	cfg.MaxConns = 4
	cfg.MinConns = 0
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, ErrConnection
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, ErrConnection
	}
	return pool, nil
}

func readLocalPassword(passwordFile string) (string, error) {
	info, err := os.Lstat(passwordFile)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() < 1 || info.Size() > 4096 {
		return "", ErrConnection
	}
	secret, err := os.ReadFile(passwordFile)
	if err != nil {
		return "", ErrConnection
	}
	defer clear(secret)
	password := strings.TrimSuffix(string(secret), "\n")
	if password == "" || strings.ContainsAny(password, "\r\n") {
		return "", ErrConnection
	}
	return password, nil
}
