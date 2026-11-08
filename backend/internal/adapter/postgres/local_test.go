package postgres

import (
	"context"
	"github.com/smofktn/watchbrief/backend/internal/domain"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalPort(t *testing.T) {
	for _, tc := range []struct{ value, want string }{
		{"", "5432"}, {"1", "1"}, {"65535", "65535"}, {"05432", "5432"},
		{"0", ""}, {"65536", ""}, {"-1", ""}, {"+5432", ""}, {" 5432", ""},
		{"5432 host=external", ""}, {"５４３２", ""}, {"999999999999999999999", ""},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("DB_PORT", tc.value)
			got, err := localPort()
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatal("unexpected local port validation")
			}
		})
	}
}

func TestLocalPasswordFileBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		mode          os.FileMode
		valid         bool
	}{
		{"private", "fictional-db-secret", 0600, true}, {"trailing LF", "fictional-db-secret\n", 0400, true}, {"maximum", strings.Repeat("x", 4096), 0600, true},
		{"empty", "", 0600, false}, {"only LF", "\n", 0600, false}, {"oversize", strings.Repeat("x", 4097), 0600, false}, {"CRLF", "fictional\r\n", 0600, false}, {"embedded LF", "first\nsecond", 0600, false}, {"multiple LF", "fictional\n\n", 0600, false}, {"group readable", "fictional", 0640, false}, {"world readable", "fictional", 0604, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "secret")
			if err := os.WriteFile(path, []byte(tc.content), tc.mode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, tc.mode); err != nil {
				t.Fatal(err)
			}
			got, err := readLocalPassword(path)
			if tc.valid {
				if err != nil || got != strings.TrimSuffix(tc.content, "\n") {
					t.Fatal("valid secret file rejected or value changed")
				}
			} else if got != "" || err != ErrConnection {
				t.Fatal("unsafe file returned a secret or unsanitized error")
			}
		})
	}
	t.Run("symlink", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "target")
		if err := os.WriteFile(target, []byte("fictional"), 0600); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if got, err := readLocalPassword(link); got != "" || err != ErrConnection {
			t.Fatal("symlink accepted")
		}
	})
	for _, path := range []string{t.TempDir(), filepath.Join(t.TempDir(), "missing")} {
		if got, err := readLocalPassword(path); got != "" || err != ErrConnection {
			t.Fatal("non-file accepted")
		}
	}
}

func TestLocalConnectionRejectsConfigurationBeforeNetwork(t *testing.T) {
	for _, scenario := range []string{"invalid port", "missing secret"} {
		t.Run(scenario, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "missing")
			t.Setenv("DB_PORT", "5432")
			if scenario == "invalid port" {
				t.Setenv("DB_PORT", "5432 host=external")
			}
			ctx := context.Background()
			if conn, err := openLocal(ctx, "fictional-role", path); conn != nil || err != ErrConnection {
				t.Fatal("unsafe connection configuration accepted")
			}
			if pool, err := OpenAPI(ctx, path); pool != nil || err != ErrConnection {
				t.Fatal("unsafe pool configuration accepted")
			}
			if err := NewLocalUserCreator(path).Create(ctx, domain.User{}); err != domain.ErrUserStore {
				t.Fatal("creator exposed connection error")
			}
			if err := NewLocalPasswordResetter(path).ResetPassword(ctx, "fictional", "fictional-phc"); err != domain.ErrUserStore {
				t.Fatal("resetter exposed connection error")
			}
		})
	}
}
