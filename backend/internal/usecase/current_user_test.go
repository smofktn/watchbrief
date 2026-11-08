package usecase

import (
	"context"
	"errors"
	"github.com/smofktn/watchbrief/backend/internal/domain"
	"testing"
)

type currentUserFunc func(context.Context, [32]byte) (string, error)

func (f currentUserFunc) LoginIDBySessionHash(c context.Context, d [32]byte) (string, error) {
	return f(c, d)
}
func TestCurrentUserBoundary(t *testing.T) {
	for _, tc := range []struct {
		name                                string
		storeErr                            error
		cancelBefore, cancelAfter, nilStore bool
	}{
		{name: "authenticated"}, {name: "unauthenticated", storeErr: domain.ErrUnauthenticated}, {name: "store failure", storeErr: domain.ErrSessionStore}, {name: "cancelled before", cancelBefore: true}, {name: "cancelled after", cancelAfter: true}, {name: "missing dependency", nilStore: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			digest := [32]byte{42}
			uc := CurrentUser{Store: currentUserFunc(func(c context.Context, d [32]byte) (string, error) {
				calls++
				if c != ctx || d != digest {
					t.Fatal("session context/digest changed")
				}
				if tc.cancelAfter {
					cancel()
				}
				if tc.storeErr != nil {
					return "", tc.storeErr
				}
				return "fictional_user", nil
			})}
			if tc.nilStore {
				uc.Store = nil
			}
			if tc.cancelBefore {
				cancel()
			}
			got, err := uc.Execute(ctx, digest)
			want := tc.storeErr
			if tc.cancelBefore || tc.cancelAfter || tc.nilStore {
				want = domain.ErrSessionStore
			}
			if !errors.Is(err, want) {
				t.Fatalf("error = %v; want %v", err, want)
			}
			if want != nil && got != "" {
				t.Fatal("identity returned on failure")
			}
			if want == nil && got != "fictional_user" {
				t.Fatal("identity mapping failed")
			}
			expected := 1
			if tc.cancelBefore || tc.nilStore {
				expected = 0
			}
			if calls != expected {
				t.Fatal("unexpected store access")
			}
		})
	}
}
