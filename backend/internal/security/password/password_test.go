package password

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func TestIndependentReference(t *testing.T) {
	data, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct{ Password, Encoded string }
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	matched, err := NewHasher().Verify(context.Background(), vector.Password, vector.Encoded)
	if err != nil || !matched {
		t.Fatal("independent Argon2id reference did not match")
	}
}

func TestHashAndVerify(t *testing.T) {
	h := NewHasher()
	ctx := context.Background()
	plain := "  日本語のパスワードをそのまま扱う Aa  "
	encoded, err := h.Hash(ctx, plain)
	if err != nil {
		t.Fatal("hash failed")
	}
	other, err := h.Hash(ctx, plain)
	if err != nil || encoded == other {
		t.Fatal("fresh salts must produce distinct hashes")
	}
	for _, tc := range []struct {
		name, input string
		want        bool
	}{
		{"exact", plain, true},
		{"trim changes password", strings.TrimSpace(plain), false},
		{"case changes password", strings.ToLower(plain), false},
		{"wrong password", "different-only-password", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := h.Verify(ctx, tc.input, encoded)
			if err != nil || got != tc.want {
				t.Fatal("unexpected password comparison")
			}
		})
	}
}

func TestRejectInvalidStoredHashes(t *testing.T) {
	data, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct{ Password, Encoded string }
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	for _, encoded := range []string{
		"", strings.Repeat("x", 10000),
		strings.Replace(vector.Encoded, "argon2id", "argon2i", 1),
		strings.Replace(vector.Encoded, "v=19", "v=16", 1),
		strings.Replace(vector.Encoded, "m=65536", "m=99999", 1),
		strings.Replace(vector.Encoded, "t=3", "t=0", 1),
		strings.Replace(vector.Encoded, "p=4", "p=0", 1),
		vector.Encoded + "=", vector.Encoded + "\n",
		strings.Replace(vector.Encoded, "$AAE", "$!AE", 1),
	} {
		_, err := NewHasher().Verify(context.Background(), vector.Password, encoded)
		if !errors.Is(err, ErrInvalidHash) {
			t.Fatal("malformed or unbounded hash parameters were not rejected")
		}
	}
}

func TestNFCEquivalentPasswordsMatch(t *testing.T) {
	h := NewHasher()
	encoded, err := h.Hash(context.Background(), strings.Repeat("か\u3099", 15))
	if err != nil {
		t.Fatal("NFC hashing failed")
	}
	matched, err := h.Verify(context.Background(), strings.Repeat("が", 15), encoded)
	if err != nil || !matched {
		t.Fatal("NFC equivalent password did not match")
	}
	matched, err = h.Verify(context.Background(), strings.Repeat("ガ", 15), encoded)
	if err != nil || matched {
		t.Fatal("NFC must not fold different characters")
	}
}

func TestInputValidationAndCancellation(t *testing.T) {
	h := NewHasher()
	for _, plain := range []string{strings.Repeat("あ", 14), strings.Repeat("😀", 129), "invalid-utf8-\xff-password"} {
		if _, err := h.Hash(context.Background(), plain); !errors.Is(err, domain.ErrInvalidCredentialsInput) {
			t.Fatal("invalid input was accepted")
		}
	}
	// Occupy both slots: a cancelled request must leave the queue without hashing.
	h.slots <- struct{}{}
	h.slots <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.Hash(ctx, "example-only-password"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled operation was not rejected")
	}
	if len(h.slots) != 2 {
		t.Fatal("queue ownership changed on cancellation")
	}
	var uninitialized Hasher
	if _, err := uninitialized.Hash(context.Background(), "example-only-password"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("uninitialized hasher did not fail safely")
	}
}

func BenchmarkHash(b *testing.B) {
	h := NewHasher()
	for b.Loop() {
		if _, err := h.Hash(context.Background(), "benchmark-only-password"); err != nil {
			b.Fatal("hash failed")
		}
	}
}
