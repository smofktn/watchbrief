package domain

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var ErrInvalidCredentialsInput = errors.New("invalid credentials input")

// ValidateLoginInput applies the same input rules as account provisioning.
func ValidateLoginInput(loginID, password string) (string, error) {
	id, err := NormalizeLoginID(loginID)
	if err != nil {
		return "", err
	}
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	return id, nil
}

func NormalizeLoginID(loginID string) (string, error) {
	if len(loginID) < 1 || len(loginID) > 64 {
		return "", ErrInvalidCredentialsInput
	}
	id := []byte(loginID)
	for i, c := range id {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
			id[i] = c
		}
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return "", ErrInvalidCredentialsInput
		}
	}
	return string(id), nil
}

// ValidatePassword is shared by account provisioning and login validation.
func ValidatePassword(password string) error {
	_, err := NormalizePassword(password)
	return err
}

// NormalizePassword counts code points after NFC, preserving case and whitespace.
func NormalizePassword(password string) (string, error) {
	// Bound raw work, while allowing decomposed forms to compose below the limit.
	if len(password) > 4096 || !utf8.ValidString(password) {
		return "", ErrInvalidCredentialsInput
	}
	password = norm.NFC.String(password)
	n := utf8.RuneCountInString(password)
	if n < 15 || n > 128 {
		return "", ErrInvalidCredentialsInput
	}
	return password, nil
}
