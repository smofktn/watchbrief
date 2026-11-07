package usecase

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type loginStoreFake struct {
	findErr, saveErr error
	findCalls        int
	sessions         []domain.Session
}

func (s *loginStoreFake) ByLoginID(context.Context, string) (domain.User, error) {
	s.findCalls++
	return domain.User{ID: "user-id", LoginID: "user", PasswordHash: "stored-hash"}, s.findErr
}
func (s *loginStoreFake) CreateSession(_ context.Context, session domain.Session, expectedHash string) error {
	if expectedHash != "stored-hash" {
		panic("missing verified hash")
	}
	s.sessions = append(s.sessions, session)
	return s.saveErr
}

type verifierFake struct {
	match   bool
	err     error
	encoded []string
}

func (v *verifierFake) Verify(_ context.Context, _ string, encoded string) (bool, error) {
	v.encoded = append(v.encoded, encoded)
	return v.match, v.err
}

func TestLoginSessionCreationAndFailures(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		findErr, verifyErr, saveErr error
		match, tokenFail, idFail    bool
		want                        error
		saveCalls                   int
	}{
		{"success", nil, nil, nil, true, false, false, nil, 1},
		{"unknown ID", domain.ErrUserNotFound, nil, nil, true, false, false, ErrInvalidCredentials, 0},
		{"wrong password", nil, nil, nil, false, false, false, ErrInvalidCredentials, 0},
		{"database failure", domain.ErrUserStore, nil, nil, true, false, false, ErrLogin, 0},
		{"corrupt stored hash", nil, errors.New("bad hash"), nil, false, false, false, ErrLogin, 0},
		{"token generation failure", nil, nil, nil, true, true, false, ErrLogin, 0},
		{"ID generation failure", nil, nil, nil, true, false, true, ErrLogin, 0},
		{"concurrent reset", nil, nil, domain.ErrPasswordChanged, true, false, false, ErrInvalidCredentials, 1},
		{"save failure", nil, nil, domain.ErrSessionStore, true, false, false, ErrLogin, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &loginStoreFake{findErr: tc.findErr, saveErr: tc.saveErr}
			v := &verifierFake{match: tc.match, err: tc.verifyErr}
			var raw [32]byte
			raw[0] = 7
			digest := sha256.Sum256(raw[:])
			uc := Login{Store: s, Verifier: v, DummyHash: "dummy-hash", NewID: func() (string, error) {
				if tc.idFail {
					return "", errors.New("ID failed")
				}
				return "session-id", nil
			}, NewToken: func() ([32]byte, [32]byte, error) {
				if tc.tokenFail {
					return [32]byte{}, [32]byte{}, errors.New("random failed")
				}
				return raw, digest, nil
			}}
			got, err := uc.Execute(context.Background(), "USER", "example-only-password")
			if !errors.Is(err, tc.want) || len(s.sessions) != tc.saveCalls {
				t.Fatal("incorrect outcome or session write")
			}
			if tc.want == nil && (got != raw || s.sessions[0].ID != "session-id" || s.sessions[0].UserID != "user-id" || s.sessions[0].TokenHash != digest) {
				t.Fatal("incorrect token/session mapping")
			}
			if tc.want != nil && got != [32]byte{} {
				t.Fatal("token escaped on failure")
			}
			if errors.Is(tc.findErr, domain.ErrUserNotFound) && (len(v.encoded) != 1 || v.encoded[0] != "dummy-hash") {
				t.Fatal("unknown ID skipped equivalent password verification")
			}
			if errors.Is(tc.findErr, domain.ErrUserStore) && len(v.encoded) != 0 {
				t.Fatal("DB failure reached password verification")
			}
		})
	}
}

func TestCancelledLoginDoesNotReadStore(t *testing.T) {
	s := &loginStoreFake{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := (Login{Store: s}).Execute(ctx, "user", "example-only-password")
	if !errors.Is(err, ErrLogin) || s.findCalls != 0 {
		t.Fatal("cancelled login reached store")
	}
}
