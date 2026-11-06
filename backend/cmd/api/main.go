package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres"
	"github.com/smofktn/watchbrief/backend/internal/security/identifier"
	"github.com/smofktn/watchbrief/backend/internal/security/password"
	"github.com/smofktn/watchbrief/backend/internal/security/token"
	"github.com/smofktn/watchbrief/backend/internal/transport/httpapi"
	"github.com/smofktn/watchbrief/backend/internal/usecase"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		// Errors are deliberately fixed labels; underlying driver/OS errors may contain secrets.
		log.Error("api_stopped", "operation", "startup_or_serve", "stage", err.Error(), "result", "failed", "version", "issue-53")
		os.Exit(1)
	}
}

type sanitizedServerLog struct{ logger *slog.Logger }

func (w sanitizedServerLog) Write(p []byte) (int, error) {
	w.logger.Error("http_server_failed", "operation", "http_server", "stage", "transport", "result", "failed", "version", "issue-53")
	return len(p), nil
}

func run(logger *slog.Logger) error {
	secretPath := os.Getenv("API_PASSWORD_FILE")
	if secretPath == "" {
		secretPath = "../.local/watchbrief-api-password"
	}
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 5*time.Second)
	pool, err := postgres.OpenAPI(startupCtx, secretPath)
	cancelStartup()
	if err != nil {
		return errors.New("database_connect")
	}
	defer pool.Close()
	hasher := password.NewHasher()
	var dummyPassword [32]byte
	if _, err := rand.Read(dummyPassword[:]); err != nil {
		return errors.New("dummy_hash_initialize")
	}
	dummyHash, err := hasher.Hash(context.Background(), base64.RawURLEncoding.EncodeToString(dummyPassword[:]))
	clear(dummyPassword[:])
	if err != nil {
		return errors.New("dummy_hash_initialize")
	}
	login := usecase.Login{Store: postgres.NewUserStore(pool), Verifier: hasher, DummyHash: dummyHash, NewID: identifier.NewULID, NewToken: token.New}
	handler, err := httpapi.NewHandler(os.Getenv("APP_ORIGIN"), login, usecase.CurrentUser{Store: postgres.NewUserStore(pool)}, logger)
	if err != nil {
		return errors.New("invalid_origin_configuration")
	}
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if !validLocalAddress(addr) {
		return errors.New("invalid_listen_configuration")
	}
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384, ErrorLog: log.New(sanitizedServerLog{logger: logger}, "", 0)}
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return errors.New("http_listen")
	}
	defer listener.Close()
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	logger.Info("api_started", "operation", "startup", "result", "ready", "version", "issue-53")
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return errors.New("http_serve")
		}
	case <-signals.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			return errors.New("http_shutdown")
		}
	}
	return nil
}

func validLocalAddress(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	n, err := strconv.Atoi(port)
	return err == nil && n > 0 && n <= 65535 && ip != nil && ip.IsLoopback()
}
