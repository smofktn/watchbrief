package token

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

func New() (raw [32]byte, digest [32]byte, err error) {
	if _, err := rand.Read(raw[:]); err != nil {
		return [32]byte{}, [32]byte{}, errors.New("cannot generate authentication token")
	}
	return raw, sha256.Sum256(raw[:]), nil
}
