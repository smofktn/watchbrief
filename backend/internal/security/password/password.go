// Package password implements the common Argon2id password hashing boundary.
package password

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/smofktn/watchbrief/backend/internal/domain"
	"golang.org/x/crypto/argon2"
)

const (
	memoryKiB   = 64 * 1024
	iterations  = 3
	parallelism = 4
	saltBytes   = 16
	keyBytes    = 32
	prefix      = "$argon2id$v=19$m=65536,t=3,p=4$"
)

var (
	ErrInvalidHash = errors.New("invalid password hash")
	ErrUnavailable = errors.New("password hasher unavailable")
)

// Hasher must be constructed with NewHasher and shared across requests.
// Each instance admits at most two KDF calls; this is not a login rate limiter.
type Hasher struct {
	slots chan struct{}
}

func NewHasher() *Hasher {
	return &Hasher{slots: make(chan struct{}, 2)}
}

func (h *Hasher) acquire(ctx context.Context) error {
	if h == nil || h.slots == nil {
		return ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case h.slots <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-h.slots
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	password, err := domain.NormalizePassword(password)
	if err != nil {
		return "", err
	}
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer func() { <-h.slots }()
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", ErrUnavailable
	}
	key := argon2.IDKey([]byte(password), salt, iterations, memoryKiB, parallelism, keyBytes)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return prefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}

func (h *Hasher) Verify(ctx context.Context, password, encoded string) (bool, error) {
	password, err := domain.NormalizePassword(password)
	if err != nil {
		return false, err
	}
	// Validate before doing any expensive work. A DB value cannot choose KDF costs.
	expectedLength := len(prefix) + base64.RawStdEncoding.EncodedLen(saltBytes) + 1 + base64.RawStdEncoding.EncodedLen(keyBytes)
	if len(encoded) != expectedLength || !strings.HasPrefix(encoded, prefix) {
		return false, ErrInvalidHash
	}
	parts := strings.Split(strings.TrimPrefix(encoded, prefix), "$")
	if len(parts) != 2 {
		return false, ErrInvalidHash
	}
	salt, err := decode(parts[0], saltBytes)
	if err != nil {
		return false, err
	}
	want, err := decode(parts[1], keyBytes)
	if err != nil {
		return false, err
	}
	if err := h.acquire(ctx); err != nil {
		return false, err
	}
	defer func() { <-h.slots }()
	got := argon2.IDKey([]byte(password), salt, iterations, memoryKiB, parallelism, keyBytes)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func decode(encoded string, length int) ([]byte, error) {
	value, err := base64.RawStdEncoding.Strict().DecodeString(encoded)
	if err != nil || len(value) != length || base64.RawStdEncoding.EncodeToString(value) != encoded {
		return nil, ErrInvalidHash
	}
	return value, nil
}
