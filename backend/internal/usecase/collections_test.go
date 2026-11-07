package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type collectionStoreStub struct {
	user     string
	authErr  error
	seenUser string
	calls    int
}

func (s *collectionStoreStub) UserIDBySessionHash(context.Context, [32]byte) (string, error) {
	return s.user, s.authErr
}
func (s *collectionStoreStub) ListOwnedCollections(_ context.Context, user string) ([]domain.CollectionSummary, error) {
	s.seenUser = user
	s.calls++
	return []domain.CollectionSummary{}, nil
}
func (s *collectionStoreStub) OwnedCollection(_ context.Context, user, id string) (domain.Collection, error) {
	s.seenUser = user
	s.calls++
	return domain.Collection{}, domain.ErrResourceNotFound
}

func TestCollectionsAuthenticateBeforeReading(t *testing.T) {
	s := &collectionStoreStub{authErr: domain.ErrUnauthenticated}
	uc := Collections{Store: s}
	if _, err := uc.List(context.Background(), [32]byte{}); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatal("missing auth rejection")
	}
	if _, err := uc.Get(context.Background(), [32]byte{}, "invalid"); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatal("identifier must not bypass authentication")
	}
	if s.calls != 0 {
		t.Fatal("unauthenticated resource lookup")
	}
	s.authErr = nil
	s.user = "trusted-session-owner"
	if _, err := uc.List(context.Background(), [32]byte{}); err != nil || s.seenUser != s.user {
		t.Fatal("list must use session owner")
	}
	if _, err := uc.Get(context.Background(), [32]byte{}, "invalid"); !errors.Is(err, domain.ErrInvalidIdentifier) {
		t.Fatal("invalid ID accepted")
	}
	if _, err := uc.Get(context.Background(), [32]byte{}, "01ARZ3NDEKTSV4RRFFQ69G5FAV"); !errors.Is(err, domain.ErrResourceNotFound) || s.seenUser != s.user {
		t.Fatal("lookup must use session owner")
	}
}

func TestCollectionCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &collectionStoreStub{}
	if _, err := (Collections{Store: s}).List(ctx, [32]byte{}); !errors.Is(err, domain.ErrCollectionStore) {
		t.Fatal("cancelled request succeeded")
	}
}
