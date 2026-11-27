package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/smofktn/watchbrief/backend/internal/domain"
)

type homeStoreStub struct {
	userErr   error
	listErr   error
	userID    string
	listCalls int
	rows      []domain.HomeCollection
}

func (s *homeStoreStub) UserIDBySessionHash(context.Context, [32]byte) (string, error) {
	return "home-owner", s.userErr
}

func (s *homeStoreStub) ListHomeCollections(_ context.Context, userID string) ([]domain.HomeCollection, error) {
	s.listCalls++
	s.userID = userID
	return s.rows, s.listErr
}

func TestHomeAuthenticatesBeforeQueryValidation(t *testing.T) {
	store := &homeStoreStub{userErr: domain.ErrUnauthenticated}
	_, err := (Home{Store: store}).List(context.Background(), [32]byte{}, "unexpected=private")
	if !errors.Is(err, domain.ErrUnauthenticated) || store.listCalls != 0 {
		t.Fatal("authentication must precede query validation and home reads")
	}
}

func TestHomeRejectsQueryAndScopesToSessionOwner(t *testing.T) {
	store := &homeStoreStub{rows: []domain.HomeCollection{{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Name: "current"}}}
	uc := Home{Store: store}
	if _, err := uc.List(context.Background(), [32]byte{}, "cursor=bad"); !errors.Is(err, domain.ErrHomeRequest) || store.listCalls != 0 {
		t.Fatal("unsupported query must be rejected after identity resolution and before storage")
	}
	value, err := uc.List(context.Background(), [32]byte{}, "")
	if err != nil || store.userID != "home-owner" || len(value.Collections) != 1 || value.Collections[0].LatestReportID != nil {
		t.Fatal("home read was not scoped or returned the wrong contract")
	}
	store.rows = nil
	value, err = uc.List(context.Background(), [32]byte{}, "")
	if err != nil || value.Collections == nil || len(value.Collections) != 0 {
		t.Fatal("no Collections must return an empty array")
	}
}
