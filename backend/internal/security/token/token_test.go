package token

import (
	"crypto/sha256"
	"testing"
)

func TestIndependentTokensAndDigests(t *testing.T) {
	seen := map[[32]byte]bool{}
	for range 32 {
		raw, digest, err := New()
		if err != nil {
			t.Fatal(err)
		}
		if raw == [32]byte{} || seen[raw] {
			t.Fatal("token reused or empty")
		}
		seen[raw] = true
		if digest != sha256.Sum256(raw[:]) || raw == digest {
			t.Fatal("stored token digest does not match SHA256")
		}
	}
}
