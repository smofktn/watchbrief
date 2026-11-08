package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type fakeWriter struct {
	users []domain.User
	err   error
}

func (s *fakeWriter) Create(_ context.Context, u domain.User) error {
	s.users = append(s.users, u)
	return s.err
}

type fakeHasher struct {
	inputs []string
	err    error
}

func (h *fakeHasher) Hash(_ context.Context, p string) (string, error) {
	h.inputs = append(h.inputs, p)
	return "synthetic-hash", h.err
}

func TestCreateUserNFCAndOrdering(t *testing.T) {
	s := &fakeWriter{}
	h := &fakeHasher{}
	uc := CreateUser{Store: s, Hasher: h, NewID: func() (string, error) { return "01ARZ3NDEKTSV4RRFFQ69G5FAV", nil }}
	if err := uc.Execute(context.Background(), "Fictional_User", strings.Repeat("か\u3099", 15), strings.Repeat("が", 15)); err != nil {
		t.Fatal("equivalent NFC confirmation should succeed")
	}
	if len(s.users) != 1 || s.users[0].LoginID != "fictional_user" || s.users[0].PasswordHash != "synthetic-hash" || h.inputs[0] != strings.Repeat("が", 15) {
		t.Fatal("incorrect normalized data or stored hash")
	}
}

func TestCreateUserStopsBeforeWrite(t *testing.T) {
	for _, tc := range []struct {
		name, id, password, confirmation string
		hashError, idError               bool
	}{
		{"invalid ID", "invalid id", "example-only-password", "example-only-password", false, false},
		{"short password", "user", "short", "short", false, false},
		{"mismatch", "user", "example-only-password", "different-only-password", false, false},
		{"hash failure", "user", "example-only-password", "example-only-password", true, false},
		{"ID failure", "user", "example-only-password", "example-only-password", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakeWriter{}
			h := &fakeHasher{}
			if tc.hashError {
				h.err = errors.New("synthetic failure")
			}
			uc := CreateUser{Store: s, Hasher: h, NewID: func() (string, error) {
				if tc.idError {
					return "", errors.New("synthetic failure")
				}
				return "id", nil
			}}
			if err := uc.Execute(context.Background(), tc.id, tc.password, tc.confirmation); err == nil || len(s.users) != 0 {
				t.Fatal("failure must not write user")
			}
		})
	}
}

func TestCreateUserPreservesDuplicateClassification(t *testing.T) {
	s := &fakeWriter{err: domain.ErrDuplicateLoginID}
	uc := CreateUser{Store: s, Hasher: &fakeHasher{}, NewID: func() (string, error) { return "id", nil }}
	if err := uc.Execute(context.Background(), "user", "example-only-password", "example-only-password"); !errors.Is(err, domain.ErrDuplicateLoginID) {
		t.Fatal("lost duplicate classification")
	}
}

func TestCreateUserMissingDependenciesAndInvalidConfirmation(t *testing.T) {
	for _, missing := range []string{"store", "hasher", "ID", "confirmation"} {
		t.Run(missing, func(t *testing.T) {
			s := &fakeWriter{}
			h := &fakeHasher{}
			ids := 0
			uc := CreateUser{Store: s, Hasher: h, NewID: func() (string, error) { ids++; return "id", nil }}
			confirmation := "example-only-password"
			want := ErrCreateUser
			switch missing {
			case "store":
				uc.Store = nil
			case "hasher":
				uc.Hasher = nil
			case "ID":
				uc.NewID = nil
			case "confirmation":
				confirmation = "short"
				want = domain.ErrInvalidCredentialsInput
			}
			if err := uc.Execute(context.Background(), "user", "example-only-password", confirmation); err != want || len(s.users) != 0 || len(h.inputs) != 0 || ids != 0 {
				t.Fatal("invalid request reached preparation/persistence")
			}
		})
	}
}
