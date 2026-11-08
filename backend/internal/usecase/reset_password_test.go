package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type resetterFake struct {
	id, hash string
	calls    int
	err      error
}

func (s *resetterFake) ResetPassword(_ context.Context, id, hash string) error {
	s.id, s.hash = id, hash
	s.calls++
	return s.err
}
func TestResetPasswordValidationAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name, id, password, confirmation string
		hashErr, storeErr, want          error
		write                            bool
	}{
		{"NFC", "USER", strings.Repeat("か\u3099", 15), strings.Repeat("が", 15), nil, nil, nil, true},
		{"invalid ID", "bad id", "example-only-password", "example-only-password", nil, nil, domain.ErrInvalidCredentialsInput, false},
		{"short", "user", "short", "short", nil, nil, domain.ErrInvalidCredentialsInput, false},
		{"confirmation", "user", "example-only-password", "different-password", nil, nil, ErrConfirmation, false},
		{"hash error", "user", "example-only-password", "example-only-password", errors.New("synthetic"), nil, ErrResetPassword, false},
		{"missing user", "user", "example-only-password", "example-only-password", nil, domain.ErrUserNotFound, domain.ErrUserNotFound, true},
		{"store error", "user", "example-only-password", "example-only-password", nil, domain.ErrUserStore, domain.ErrUserStore, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &resetterFake{err: tc.storeErr}
			h := &fakeHasher{err: tc.hashErr}
			err := (ResetPassword{Store: s, Hasher: h}).Execute(context.Background(), tc.id, tc.password, tc.confirmation)
			if !errors.Is(err, tc.want) || (s.calls == 1) != tc.write {
				t.Fatal("unexpected reset outcome/write")
			}
			if tc.write && (s.id != "user" || s.hash != "synthetic-hash") {
				t.Fatal("invalid normalized/persisted values")
			}
			if tc.name == "NFC" && h.inputs[0] != strings.Repeat("が", 15) {
				t.Fatal("password not NFC normalized")
			}
		})
	}
	s := &resetterFake{}
	h := &fakeHasher{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (ResetPassword{Store: s, Hasher: h}).Execute(ctx, "user", "example-only-password", "example-only-password"); !errors.Is(err, ErrResetPassword) || s.calls != 0 || len(h.inputs) != 0 {
		t.Fatal("cancelled reset performed work")
	}
}

func TestResetPasswordMissingDependenciesAndInvalidConfirmation(t *testing.T) {
	for _, missing := range []string{"store", "hasher", "confirmation"} {
		t.Run(missing, func(t *testing.T) {
			s := &resetterFake{}
			h := &fakeHasher{}
			uc := ResetPassword{Store: s, Hasher: h}
			confirmation := "example-only-password"
			want := ErrResetPassword
			switch missing {
			case "store":
				uc.Store = nil
			case "hasher":
				uc.Hasher = nil
			case "confirmation":
				confirmation = "short"
				want = domain.ErrInvalidCredentialsInput
			}
			if err := uc.Execute(context.Background(), "user", "example-only-password", confirmation); err != want || s.calls != 0 || len(h.inputs) != 0 {
				t.Fatal("invalid reset reached hashing/persistence")
			}
		})
	}
}
