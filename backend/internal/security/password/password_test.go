package password

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestUnavailableHasherAndQueuedCancellation(t *testing.T) {
	for _, h := range []*Hasher{nil, {}} {
		if _, err := h.Hash(context.Background(), "example-only-password"); err != ErrUnavailable {
			t.Fatal("unavailable hasher accepted input")
		}
	}
	h := NewHasher()
	h.slots <- struct{}{}
	h.slots <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	observed := &admissionContext{Context: ctx, entered: started}
	result := make(chan error, 1)
	go func() { result <- h.acquire(observed) }()
	select {
	case <-started:
	case err := <-result:
		t.Fatalf("queue did not block: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("waiter did not reach admission queue")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("queued request did not cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("queued request remained blocked")
	}
	if len(h.slots) != 2 {
		t.Fatal("cancelled waiter changed queue ownership")
	}
	<-h.slots
	<-h.slots
	if err := h.acquire(context.Background()); err != nil {
		t.Fatal("queue unusable after cancellation")
	}
	<-h.slots
}
func TestVerifyRejectsMalformedBase64BeforeHashing(t *testing.T) {
	data, err := os.ReadFile("testdata/reference.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct{ Password, Encoded string }
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(vector.Encoded, "$")
	key := parts[len(parts)-1]
	for _, encoded := range []string{strings.TrimSuffix(vector.Encoded, key) + "!" + key[1:], strings.TrimSuffix(vector.Encoded, key) + key[:len(key)-1] + "=", strings.Replace(vector.Encoded, "$AAE", "$AA$", 1)} {
		if matched, err := (*Hasher)(nil).Verify(context.Background(), vector.Password, encoded); matched || err != ErrInvalidHash {
			t.Fatal("invalid hash reached unavailable KDF")
		}
	}
	if matched, err := (*Hasher)(nil).Verify(context.Background(), vector.Password, vector.Encoded); matched || err != ErrUnavailable {
		t.Fatal("valid profile must still require initialized KDF")
	}
	if matched, err := NewHasher().Verify(context.Background(), "short", vector.Encoded); matched || err != domain.ErrInvalidCredentialsInput {
		t.Fatal("invalid verification input accepted")
	}
}

// Done is evaluated when acquire reaches its select with both slots occupied.
type admissionContext struct {
	context.Context
	entered chan struct{}
	once    sync.Once
}

func (c *admissionContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.entered) })
	return c.Context.Done()
}
