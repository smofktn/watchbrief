package domain

import "errors"

// User is internal application data, not an HTTP response or log value.
type User struct {
	ID           string
	LoginID      string
	PasswordHash string
}

var (
	ErrDuplicateLoginID = errors.New("login ID already exists")
	ErrPasswordChanged  = errors.New("password changed during login")
	ErrUserStore        = errors.New("user storage operation failed")
)
