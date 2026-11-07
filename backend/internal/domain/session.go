package domain

import "errors"

type Session struct {
	ID        string
	UserID    string
	TokenHash [32]byte
}

var (
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrUserNotFound    = errors.New("user not found")
	ErrSessionStore    = errors.New("session storage operation failed")
)
