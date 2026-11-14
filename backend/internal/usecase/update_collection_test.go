package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type updateStoreStub struct {
	auth   error
	calls  int
	owner  string
	digest [32]byte
}

func (s *updateStoreStub) UserIDBySessionHash(context.Context, [32]byte) (string, error) {
	return "session-owner", s.auth
}
func (s *updateStoreStub) UpdateOwnedCollection(_ context.Context, owner, id, key string, digest [32]byte, input domain.CollectionUpdate) (domain.CollectionUpdateResult, error) {
	s.calls++
	s.owner = owner
	s.digest = digest
	return domain.CollectionUpdateResult{ID: id, Version: "2"}, nil
}
func TestUpdateAuthenticatesValidatesAndBindsRequest(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const key = "00112233-4455-4677-8899-aabbccddeeff"
	s := &updateStoreStub{auth: domain.ErrUnauthenticated}
	uc := UpdateCollection{Store: s}
	input := domain.CollectionUpdate{NewCollection: domain.NewCollection{Name: "架空", InitialLookbackMode: "ai"}, Version: "1"}
	if _, err := uc.Update(context.Background(), [32]byte{}, "bad", key, input); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatal("authenticate first")
	}
	s.auth = nil
	for _, v := range []string{"", "0", "01", "+1", "1.0", "9223372036854775808"} {
		bad := input
		bad.Version = v
		if _, err := uc.Update(context.Background(), [32]byte{}, id, key, bad); err == nil {
			t.Fatal("invalid version")
		}
	}
	bad := input
	bad.Name = "　 "
	if _, err := uc.Update(context.Background(), [32]byte{}, id, key, bad); err == nil {
		t.Fatal("invalid name")
	}
	if _, err := uc.Update(context.Background(), [32]byte{}, id, "bad", input); err == nil {
		t.Fatal("invalid key")
	}
	if s.calls != 0 {
		t.Fatal("invalid input saved")
	}
	if _, err := uc.Update(context.Background(), [32]byte{}, id, key, input); err != nil || s.owner != "session-owner" {
		t.Fatal("owner")
	}
	previous := s.digest
	input.Version = "2"
	uc.Update(context.Background(), [32]byte{}, id, key, input)
	if s.digest == previous {
		t.Fatal("version not bound to digest")
	}
	previous = s.digest
	uc.Update(context.Background(), [32]byte{}, "01ARZ3NDEKTSV4RRFFQ69G5FAW", key, input)
	if s.digest == previous {
		t.Fatal("ID not bound to digest")
	}
	previous = s.digest
	input.ResearchInstruction = "変更"
	uc.Update(context.Background(), [32]byte{}, id, key, input)
	if s.digest == previous {
		t.Fatal("input not bound to digest")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := uc.Update(ctx, [32]byte{}, id, key, input); !errors.Is(err, domain.ErrCollectionStore) {
		t.Fatal("cancelled")
	}
	if _, err := (UpdateCollection{}).Update(context.Background(), [32]byte{}, id, key, input); !errors.Is(err, domain.ErrCollectionStore) {
		t.Fatal("nil store")
	}
}
