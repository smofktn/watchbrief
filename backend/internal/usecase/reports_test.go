package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type reportStoreStub struct {
	user     string
	authErr  error
	seenUser string
	calls    int
}

func (s *reportStoreStub) UserIDBySessionHash(context.Context, [32]byte) (string, error) {
	return s.user, s.authErr
}
func (s *reportStoreStub) ListOwnedReports(_ context.Context, user string) ([]domain.ReportSummary, error) {
	s.seenUser = user
	s.calls++
	return []domain.ReportSummary{}, nil
}
func (s *reportStoreStub) OwnedReport(_ context.Context, user, id string) (domain.Report, error) {
	s.seenUser = user
	s.calls++
	return domain.Report{}, domain.ErrResourceNotFound
}

func TestReportsAuthenticateBeforeReading(t *testing.T) {
	s := &reportStoreStub{authErr: domain.ErrUnauthenticated}
	uc := Reports{Store: s}
	if _, err := uc.List(context.Background(), [32]byte{}, ""); !errors.Is(err, domain.ErrUnauthenticated) {
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
	if _, err := uc.List(context.Background(), [32]byte{}, ""); err != nil || s.seenUser != s.user {
		t.Fatal("list must use session owner")
	}
	if _, err := uc.Get(context.Background(), [32]byte{}, "invalid"); !errors.Is(err, domain.ErrInvalidIdentifier) {
		t.Fatal("invalid ID accepted")
	}
	if _, err := uc.Get(context.Background(), [32]byte{}, "01ARZ3NDEKTSV4RRFFQ69G5FAV"); !errors.Is(err, domain.ErrResourceNotFound) || s.seenUser != s.user {
		t.Fatal("lookup must use session owner")
	}
}

func TestReportCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &reportStoreStub{}
	if _, err := (Reports{Store: s}).List(ctx, [32]byte{}, ""); !errors.Is(err, domain.ErrReportStore) {
		t.Fatal("cancelled request succeeded")
	}
}

func (s *reportStoreStub) ListOwnedReportsBefore(c context.Context, u string, _ domain.ReportCursor) ([]domain.ReportSummary, error) {
	return s.ListOwnedReports(c, u)
}

type pageStore struct {
	reportStoreStub
	rows   []domain.ReportSummary
	before *domain.ReportCursor
}

func (s *pageStore) ListOwnedReports(_ context.Context, user string) ([]domain.ReportSummary, error) {
	s.calls++
	s.seenUser = user
	return s.rows, nil
}
func (s *pageStore) ListOwnedReportsBefore(c context.Context, user string, cursor domain.ReportCursor) ([]domain.ReportSummary, error) {
	s.before = &cursor
	return s.ListOwnedReports(c, user)
}
func TestReportPageBoundariesAndAuthenticationFirst(t *testing.T) {
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	at := time.Date(2026, 4, 1, 0, 0, 0, 123456000, time.UTC)
	s := &pageStore{reportStoreStub: reportStoreStub{user: "session-owner"}}
	uc := Reports{Store: s}
	for _, n := range []int{0, 30, 31, 35} {
		s.rows = make([]domain.ReportSummary, n)
		for i := range s.rows {
			s.rows[i] = domain.ReportSummary{ID: id, ScheduledAt: at}
		}
		got, err := uc.List(context.Background(), [32]byte{}, "")
		if err != nil || len(got.Reports) != min(n, 30) || got.Reports == nil || (got.NextCursor != nil) != (n > 30) {
			t.Fatal("wrong boundary")
		}
	}
	token := domain.EncodeReportCursor(domain.ReportSummary{ID: id, ScheduledAt: at})
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
		if _, err := uc.List(context.Background(), [32]byte{}, query); !errors.Is(err, domain.ErrReportCursor) || calls != s.calls {
			t.Fatal("invalid query read rows")
		}
	}
}
