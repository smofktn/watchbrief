package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
)

func TestListenAddressIsLoopbackOnly(t *testing.T) {
	for _, tc := range []struct {
		addr  string
		valid bool
	}{
		{"127.0.0.1:8080", true}, {"[::1]:8080", true}, {"127.0.0.1:18080", true},
		{"0.0.0.0:8080", false}, {":8080", false}, {"192.0.2.1:8080", false}, {"127.0.0.1:0", false}, {"127.0.0.1:65536", false}, {"invalid", false},
	} {
		if validLocalAddress(tc.addr) != tc.valid {
			t.Fatal("incorrect listen address validation")
		}
	}
}

func TestServerLogDiscardsRawTransportError(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	raw := []byte("fictional-password Authorization: fictional-token")
	n, err := (sanitizedServerLog{logger: logger}).Write(raw)
	if err != nil || n != len(raw) {
		t.Fatal("server logging contract failed")
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["msg"] != "http_server_failed" || record["stage"] != "transport" || record["result"] != "failed" {
		t.Fatal("missing safe diagnostic labels")
	}
	for _, secret := range []string{"fictional-password", "Authorization", "fictional-token"} {
		if strings.Contains(output.String(), secret) {
			t.Fatal("raw transport error escaped into log")
		}
	}
}
func TestStartupMissingSecretIsSanitized(t *testing.T) {
	t.Setenv("API_PASSWORD_FILE", filepath.Join(t.TempDir(), "missing-fictional-secret"))
	t.Setenv("DB_PORT", "5432")
	var output bytes.Buffer
	err := run(slog.New(slog.NewJSONHandler(&output, nil)))
	if err == nil || err.Error() != "database_connect" || output.Len() != 0 {
		t.Fatal("startup failure exposed local connection details")
	}
}
