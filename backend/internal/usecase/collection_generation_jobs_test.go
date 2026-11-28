package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type collectionJobStoreStub struct {
	userErr      error
	exists       bool
	rows         []domain.GenerationJobSummary
	userID       string
	collectionID string
	calls        int
	before       bool
}

func (s *collectionJobStoreStub) UserIDBySessionHash(context.Context, [32]byte) (string, error) {
	return "owner", s.userErr
}
func (s *collectionJobStoreStub) ListOwnedCollectionGenerationJobs(_ context.Context, user, collection string) (bool, []domain.GenerationJobSummary, error) {
	s.calls++
	s.userID, s.collectionID = user, collection
	return s.exists, s.rows, nil
}
func (s *collectionJobStoreStub) ListOwnedCollectionGenerationJobsBefore(_ context.Context, user, collection string, _ domain.GenerationJobCursor) (bool, []domain.GenerationJobSummary, error) {
	s.before = true
	return s.ListOwnedCollectionGenerationJobs(context.Background(), user, collection)
}

const collectionJobTestID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

func TestCollectionGenerationJobsAuthenticatesBeforeIdentifierAndQuery(t *testing.T) {
	store := &collectionJobStoreStub{userErr: domain.ErrUnauthenticated}
	uc := CollectionGenerationJobs{Store: store}
	for _, tc := range []struct{ id, query string }{{"bad", "cursor=%xx"}, {collectionJobTestID, "cursor=%xx"}} {
		if _, err := uc.List(context.Background(), [32]byte{}, tc.id, tc.query); !errors.Is(err, domain.ErrUnauthenticated) {
			t.Fatal("authentication must precede ID and query validation")
		}
	}
	if store.calls != 0 {
		t.Fatal("unauthenticated request reached Collection query")
	}
}

func TestCollectionGenerationJobsScopesAndPaginates(t *testing.T) {
	rows := make([]domain.GenerationJobSummary, 31)
	for i := range rows {
		rows[i] = domain.GenerationJobSummary{
			ID:             fmt.Sprintf("01ARZ3NDEKTSV4RRFFQ69G5F%02X", i),
			CollectionName: "fictional history",
			ScheduledAt:    time.Date(2026, 11, 27, 3, 4, 5, int(30-i), time.UTC),
			Status:         "queued",
		}
	}
	store := &collectionJobStoreStub{exists: true, rows: rows}
	uc := CollectionGenerationJobs{Store: store}
	page, err := uc.List(context.Background(), [32]byte{}, collectionJobTestID, "")
	if err != nil || store.userID != "owner" || store.collectionID != collectionJobTestID || len(page.GenerationJobs) != 30 || page.NextCursor == nil {
		t.Fatal("first Collection page was not scoped/paginated")
	}
	if _, err := uc.List(context.Background(), [32]byte{}, collectionJobTestID, "cursor=bad"); !errors.Is(err, domain.ErrGenerationJobCursor) || store.calls != 1 {
		t.Fatal("invalid cursor reached storage")
	}
	query := "cursor=" + *page.NextCursor
	if _, err := uc.List(context.Background(), [32]byte{}, collectionJobTestID, query); err != nil || !store.before || store.calls != 2 {
		t.Fatal("valid continuation cursor was not accepted")
	}
}

func TestCollectionGenerationJobsHidesUnavailableAndKeepsEmptyDistinct(t *testing.T) {
	store := &collectionJobStoreStub{}
	uc := CollectionGenerationJobs{Store: store}
	if _, err := uc.List(context.Background(), [32]byte{}, "00000000000000000000000021", ""); !errors.Is(err, domain.ErrResourceNotFound) {
		t.Fatal("unavailable Collection must be not found")
	}
	store.exists = true
	page, err := uc.List(context.Background(), [32]byte{}, collectionJobTestID, "")
	if err != nil || page.GenerationJobs == nil || len(page.GenerationJobs) != 0 || page.NextCursor != nil {
		t.Fatal("existing Collection must return a valid empty page")
	}
}
