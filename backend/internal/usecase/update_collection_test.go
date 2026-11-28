package usecase

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
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

func TestUpdateTargetValidationAndLegacyDigest(t *testing.T) {
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const key = "00112233-4455-4677-8899-aabbccddeeff"
	s := &updateStoreStub{}
	uc := UpdateCollection{Store: s}
	input := domain.CollectionUpdate{NewCollection: domain.NewCollection{Name: "架空", InitialLookbackMode: "ai"}, Version: "1"}
	uc.Update(context.Background(), [32]byte{}, id, key, input)
	old := sha256.Sum256([]byte(`{"id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","name":"架空","research_instruction":"","initial_lookback_mode":"ai","initial_lookback_days":null,"version":"1"}`))
	if s.digest != old {
		t.Fatal("legacy digest changed")
	}
	input.Targets = []domain.TargetChange{{Name: "対象", Instruction: "補足"}}
	if _, err := uc.Update(context.Background(), [32]byte{}, id, key, input); err != nil || s.digest == old {
		t.Fatal("target not bound")
	}
	for _, rows := range [][]domain.TargetChange{
		{{Name: "　"}}, {{Name: strings.Repeat("a", 201)}}, {{Name: "x", Instruction: strings.Repeat("a", 10001)}}, {{Name: "a\x00"}},
		{{ID: &id, Name: "x"}, {ID: &id, Name: "y"}}, make([]domain.TargetChange, 21),
	} {
		input.Targets = rows
		calls := s.calls
		if _, err := uc.Update(context.Background(), [32]byte{}, id, key, input); err == nil || s.calls != calls {
			t.Fatal("invalid targets reached storage")
		}
	}
}

func TestSourceDigestBindingAndTargetCompatibility(t *testing.T) {
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const key = "00112233-4455-4677-8899-aabbccddeeff"
	s := &updateStoreStub{}
	uc := UpdateCollection{Store: s}
	input := domain.CollectionUpdate{NewCollection: domain.NewCollection{Name: "架空", InitialLookbackMode: "ai"}, Version: "1", Targets: []domain.TargetChange{{Name: "対象", Instruction: ""}}}
	uc.Update(context.Background(), [32]byte{}, id, key, input)
	expected := sha256.Sum256([]byte(`{"id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","name":"架空","research_instruction":"","initial_lookback_mode":"ai","initial_lookback_days":null,"version":"1","targets":[{"id":null,"name":"対象","instruction":""}]}`))
	if s.digest != expected {
		t.Fatal("legacy target digest changed")
	}
	input.Sources = []domain.SourceChange{{WatchTargetID: id, Name: "情報源", URL: "https://example.com", Instruction: ""}}
	uc.Update(context.Background(), [32]byte{}, id, key, input)
	previous := s.digest
	if previous == expected {
		t.Fatal("source not bound")
	}
	input.Sources[0].URL = "https://example.com/new"
	uc.Update(context.Background(), [32]byte{}, id, key, input)
	if previous == s.digest {
		t.Fatal("URL not bound")
	}
	previous = s.digest
	input.Sources[0].WatchTargetID = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	uc.Update(context.Background(), [32]byte{}, id, key, input)
	if previous == s.digest {
		t.Fatal("source parent not bound")
	}
	previous = s.digest
	input.Sources[0].ID = &id
	uc.Update(context.Background(), [32]byte{}, id, key, input)
	if previous == s.digest {
		t.Fatal("source ID not bound")
	}
	input.Sources[0].URL = "https://user:pass@example.com"
	calls := s.calls
	if _, err := uc.Update(context.Background(), [32]byte{}, id, key, input); err == nil || s.calls != calls {
		t.Fatal("invalid source reached storage")
	}
}

func TestScheduleDigestAndValidation(t *testing.T) {
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const key = "00112233-4455-4677-8899-aabbccddeeff"
	s := &updateStoreStub{}
	uc := UpdateCollection{Store: s}
	input := domain.CollectionUpdate{NewCollection: domain.NewCollection{Name: "架空", InitialLookbackMode: "ai"}, Version: "1", Sources: []domain.SourceChange{{WatchTargetID: id, Name: "情報源", URL: "https://example.com", Instruction: ""}}}
	uc.Update(context.Background(), [32]byte{}, id, key, input)
	expected := sha256.Sum256([]byte(`{"id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","name":"架空","research_instruction":"","initial_lookback_mode":"ai","initial_lookback_days":null,"version":"1","sources":[{"id":null,"watch_target_id":"01ARZ3NDEKTSV4RRFFQ69G5FAV","name":"情報源","url":"https://example.com","instruction":""}]}`))
	if s.digest != expected {
		t.Fatal("legacy source digest changed")
	}
	input.Schedule = &domain.ScheduleChange{CronExpression: "0 7 * * *"}
	if _, err := uc.Update(context.Background(), [32]byte{}, id, key, input); err != nil || s.digest == expected {
		t.Fatal("schedule not bound")
	}
	previous := s.digest
	input.Schedule.ID = &id
	uc.Update(context.Background(), [32]byte{}, id, key, input)
	if s.digest == previous {
		t.Fatal("schedule ID not bound")
	}
	previous = s.digest
	input.Schedule.CronExpression = "0 8 * * *"
	uc.Update(context.Background(), [32]byte{}, id, key, input)
	if s.digest == previous {
		t.Fatal("expression not bound")
	}
	for _, value := range []string{"", "0 25 * * *", "@daily"} {
		input.Schedule.CronExpression = value
		calls := s.calls
		if _, err := uc.Update(context.Background(), [32]byte{}, id, key, input); err == nil || calls != s.calls {
			t.Fatal("invalid schedule stored")
		}
	}
	input.Schedule.CronExpression = "0 7 * * *"
	bad := "bad"
	input.Schedule.ID = &bad
	if _, err := uc.Update(context.Background(), [32]byte{}, id, key, input); err == nil {
		t.Fatal("invalid schedule ID")
	}
}
