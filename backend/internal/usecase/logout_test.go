package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type logoutStoreFake struct {
	calls  int
	err    error
	cancel context.CancelFunc
}

func (s *logoutStoreFake) DeleteSessionByHash(context.Context, [32]byte) error {
	s.calls++
	if s.cancel != nil {
		s.cancel()
	}
	return s.err
}
func TestLogoutFailuresAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		name           string
		failure        error
		before, during bool
	}{
		{"success including absent row", nil, false, false},
		{"failure sanitized", errors.New("private SQL details"), false, false},
		{"cancel before deletion", nil, true, false},
		{"cancel during deletion", nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			store := &logoutStoreFake{err: tc.failure}
			if tc.before {
				cancel()
			}
			if tc.during {
				store.cancel = cancel
			}
			err := (Logout{Store: store}).Execute(ctx, [32]byte{})
			failed := tc.failure != nil || tc.before || tc.during
			if failed && err != domain.ErrSessionStore || !failed && err != nil {
				t.Fatal("incorrect deletion result")
			}
			expectedCalls := 1
			if tc.before {
				expectedCalls = 0
			}
			if store.calls != expectedCalls {
				t.Fatal("cancelled operation reached store")
			}
		})
	}
	if err := (Logout{}).Execute(context.Background(), [32]byte{}); err != domain.ErrSessionStore {
		t.Fatal("nil store accepted")
	}
}
