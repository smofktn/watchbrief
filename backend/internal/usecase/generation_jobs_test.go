package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type jobStoreStub struct {
	user     string
	authErr  error
	seenUser string
	calls    int
}

func (s *jobStoreStub) UserIDBySessionHash(context.Context, [32]byte) (string, error) {
	return s.user, s.authErr
}
func (s *jobStoreStub) ListOwnedGenerationJobs(_ context.Context, user string) ([]domain.GenerationJobSummary, error) {
	s.seenUser = user
	s.calls++
	return []domain.GenerationJobSummary{}, nil
}

func TestGenerationJobsAuthenticateBeforeReading(t *testing.T) {
	s := &jobStoreStub{authErr: domain.ErrUnauthenticated}
	uc := GenerationJobs{Store: s}
	if _, err := uc.List(context.Background(), [32]byte{}, ""); !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatal("missing auth rejection")
	}
	if s.calls != 0 {
		t.Fatal("unauthenticated resource lookup")
	}
	s.authErr = nil
	s.user = "trusted-session-owner"
	if _, err := uc.List(context.Background(), [32]byte{}, ""); err != nil || s.seenUser != s.user {
		t.Fatal("list must use session owner")
	}
}

func TestGenerationJobCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &jobStoreStub{}
	if _, err := (GenerationJobs{Store: s}).List(ctx, [32]byte{}, ""); !errors.Is(err, domain.ErrGenerationJobStore) {
		t.Fatal("cancelled request succeeded")
	}
}

func (s *jobStoreStub) ListOwnedGenerationJobsBefore(c context.Context, u string, _ domain.GenerationJobCursor) ([]domain.GenerationJobSummary, error) {
	return s.ListOwnedGenerationJobs(c, u)
}

type jobPageStore struct {
	jobStoreStub
	rows   []domain.GenerationJobSummary
	before *domain.GenerationJobCursor
}

func (s *jobPageStore) ListOwnedGenerationJobs(_ context.Context, user string) ([]domain.GenerationJobSummary, error) {
	s.calls++
	s.seenUser = user
	return s.rows, nil
}
func (s *jobPageStore) ListOwnedGenerationJobsBefore(c context.Context, user string, cursor domain.GenerationJobCursor) ([]domain.GenerationJobSummary, error) {
	s.before = &cursor
	return s.ListOwnedGenerationJobs(c, user)
}
func TestGenerationJobPageBoundariesAndAuthenticationFirst(t *testing.T) {
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	at := time.Date(2026, 4, 1, 0, 0, 0, 123456000, time.UTC)
	s := &jobPageStore{jobStoreStub: jobStoreStub{user: "session-owner"}}
	uc := GenerationJobs{Store: s}
	for _, n := range []int{0, 30, 31, 35} {
		s.rows = make([]domain.GenerationJobSummary, n)
		for i := range s.rows {
			s.rows[i] = domain.GenerationJobSummary{ID: id, ScheduledAt: at}
		}
		got, err := uc.List(context.Background(), [32]byte{}, "")
		if err != nil || len(got.GenerationJobs) != min(n, 30) || got.GenerationJobs == nil || (got.NextCursor != nil) != (n > 30) {
			t.Fatal("wrong boundary")
		}
	}
	token := domain.EncodeGenerationJobCursor(domain.GenerationJobSummary{ID: id, ScheduledAt: at})
	if _, err := uc.List(context.Background(), [32]byte{}, "cursor="+token); err != nil || s.seenUser != "session-owner" || s.before == nil || !s.before.ScheduledAt.Equal(at) {
		t.Fatal("cursor or owner lost")
	}
	for _, query := range []string{"cursor=", "cursor=x", "cursor=%xx", "limit=999", "cursor=" + token + "&cursor=" + token, "cursor=" + token + "&unknown=1", "&", strings.Repeat("x", 513)} {
		s.authErr = domain.ErrUnauthenticated
		if _, err := uc.List(context.Background(), [32]byte{}, query); !errors.Is(err, domain.ErrUnauthenticated) {
			t.Fatal("query bypassed authentication")
		}
		s.authErr = nil
		calls := s.calls
		if _, err := uc.List(context.Background(), [32]byte{}, query); !errors.Is(err, domain.ErrGenerationJobCursor) || calls != s.calls {
			t.Fatal("invalid query read rows")
		}
	}
}
