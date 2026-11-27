package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type homeStoreStub struct {
	userErr   error
	listErr   error
	userID    string
	listCalls int
	rows      []domain.HomeCollection
	start     time.Time
	end       time.Time
}

func (s *homeStoreStub) UserIDBySessionHash(context.Context, [32]byte) (string, error) {
	return "home-owner", s.userErr
}

func (s *homeStoreStub) ListHomeCollections(_ context.Context, userID string, start, end time.Time) ([]domain.HomeCollection, error) {
	s.listCalls++
	s.userID = userID
	s.start = start
	s.end = end
	return s.rows, s.listErr
}

func TestHomeAuthenticatesBeforeQueryValidation(t *testing.T) {
	store := &homeStoreStub{userErr: domain.ErrUnauthenticated}
	clockCalls := 0
	_, err := (Home{Store: store, Now: func() time.Time { clockCalls++; return time.Now() }}).List(context.Background(), [32]byte{}, "unexpected=private")
	if !errors.Is(err, domain.ErrUnauthenticated) || store.listCalls != 0 || clockCalls != 0 {
		t.Fatal("authentication must precede query validation and home reads")
	}
}

func TestHomeRejectsQueryAndScopesToSessionOwner(t *testing.T) {
	store := &homeStoreStub{rows: []domain.HomeCollection{{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Name: "current"}}}
	clockCalls := 0
	uc := Home{Store: store, Now: func() time.Time { clockCalls++; return time.Date(2026, 11, 27, 14, 59, 59, 999999999, time.UTC) }}
	if _, err := uc.List(context.Background(), [32]byte{}, "cursor=bad"); !errors.Is(err, domain.ErrHomeRequest) || store.listCalls != 0 || clockCalls != 0 {
		t.Fatal("unsupported query must be rejected after identity resolution and before storage")
	}
	value, err := uc.List(context.Background(), [32]byte{}, "")
	start := time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 27, 15, 0, 0, 0, time.UTC)
	if err != nil || store.userID != "home-owner" || len(value.Collections) != 1 || value.Collections[0].LatestReportID != nil || value.AsOfDate != "2026-11-27" || !store.start.Equal(start) || !store.end.Equal(end) || clockCalls != 1 {
		t.Fatal("home read was not scoped or returned the wrong contract")
	}
	store.rows = nil
	value, err = uc.List(context.Background(), [32]byte{}, "")
	if err != nil || value.Collections == nil || len(value.Collections) != 0 || value.AsOfDate != "2026-11-27" || clockCalls != 2 {
		t.Fatal("no Collections must return an empty array")
	}
}
