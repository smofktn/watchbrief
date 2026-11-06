package config

import (
 "context"
 "errors"
 "time"

 "github.com/jackc/pgx/v5/pgxpool"
)

func OpenDB(ctx context.Context, path, requiredRole string) (*pgxpool.Pool, error) {
 cfg, err := ReadDB(path)
 if err != nil { return nil, err }
 if cfg.User != requiredRole { return nil, errors.New("incorrect DB role for this command") }
 // Never format a DSN with secrets or attach a pgx query tracer.
 pc, err := pgxpool.ParseConfig("host=127.0.0.1 sslmode=disable")
 if err != nil { return nil, errors.New("cannot configure DB") }
 pc.ConnConfig.Host, pc.ConnConfig.Port = cfg.Host, cfg.Port
 pc.ConnConfig.Database, pc.ConnConfig.User, pc.ConnConfig.Password = cfg.Database, cfg.User, cfg.Password
 pc.ConnConfig.RuntimeParams = map[string]string{"timezone":"UTC", "search_path":"public,pg_catalog"}
 pc.MaxConns, pc.MinConns = 4, 0
 pc.ConnConfig.ConnectTimeout = 5 * time.Second
 pool, err := pgxpool.NewWithConfig(ctx, pc)
 if err != nil { return nil, errors.New("cannot initialize DB pool") }
 if err := pool.Ping(ctx); err != nil { pool.Close(); return nil, errors.New("cannot connect to DB") }
 return pool, nil
}
