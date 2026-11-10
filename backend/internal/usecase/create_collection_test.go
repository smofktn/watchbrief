package usecase

import (
	"context"
	"errors"
	"github.com/smofktn/watchbrief/backend/internal/domain"
	"testing"
)

type creationStoreStub struct {
	auth       error
	calls      int
	owner, key string
	digest     [32]byte
	input      domain.NewCollection
}

func (s *creationStoreStub) UserIDBySessionHash(context.Context, [32]byte) (string, error) {
	return "session-owner", s.auth
}
func (s *creationStoreStub) CreateOwnedCollection(_ context.Context, owner, key string, digest [32]byte, id string, input domain.NewCollection) (string, bool, error) {
	s.calls++
	s.owner = owner
	s.key = key
	s.digest = digest
	s.input = input
	return id, true, nil
}
func TestCreationUsesAuthenticatedOwnerAndValidatedInput(t *testing.T) {
	const key = "00112233-4455-4677-8899-aabbccddeeff"
	s := &creationStoreStub{auth: domain.ErrUnauthenticated}
	uc := CreateCollection{Store: s, NewID: func() (string, error) { return "01ARZ3NDEKTSV4RRFFQ69G5FAV", nil }}
	input := domain.NewCollection{Name: "架空", InitialLookbackMode: "ai"}
	if _, _, err := uc.Execute(context.Background(), [32]byte{}, "bad", input); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatal("authenticate before semantic validation")
	}
	s.auth = nil
	if _, _, err := uc.Execute(context.Background(), [32]byte{}, "bad", input); !errors.Is(err, domain.ErrInvalidIdentifier) {
		t.Fatal("key accepted")
	}
	if _, _, err := uc.Execute(context.Background(), [32]byte{}, key, domain.NewCollection{}); err == nil {
		t.Fatal("invalid input accepted")
	}
	if s.calls != 0 {
		t.Fatal("invalid request reached storage")
	}
	if _, created, err := uc.Execute(context.Background(), [32]byte{}, key, input); err != nil || !created || s.owner != "session-owner" || s.key != key || s.input != input {
		t.Fatal("trusted input not saved")
	}
	old := s.digest
	input.ResearchInstruction = "変更"
	if _, _, err := uc.Execute(context.Background(), [32]byte{}, key, input); err != nil || s.digest == old {
		t.Fatal("different content must have different digest")
	}
	uc.NewID = func() (string, error) { return "", errors.New("private random failure") }
	if _, _, err := uc.Execute(context.Background(), [32]byte{}, key, input); !errors.Is(err, domain.ErrCollectionStore) {
		t.Fatal("unsafe ID failure")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := uc.Execute(ctx, [32]byte{}, key, input); !errors.Is(err, domain.ErrCollectionStore) {
		t.Fatal("cancelled save")
	}
}
