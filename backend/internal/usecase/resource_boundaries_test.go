package usecase

import (
	"context"
	"github.com/smofktn/watchbrief/backend/internal/domain"
	"testing"
)

type resourceBoundaryStore struct {
	t                    *testing.T
	ctx                  context.Context
	digest               [32]byte
	wantResourceID       string
	cancel               context.CancelFunc
	cancelRead           bool
	authErr, readErr     error
	authCalls, readCalls int
}

func (s *resourceBoundaryStore) UserIDBySessionHash(ctx context.Context, digest [32]byte) (string, error) {
	s.authCalls++
	if ctx != s.ctx || digest != s.digest {
		s.t.Fatal("authentication context/digest changed")
	}
	return "trusted-owner", s.authErr
}
func (s *resourceBoundaryStore) read(ctx context.Context, owner, id string) {
	s.readCalls++
	if ctx != s.ctx || owner != "trusted-owner" || id != s.wantResourceID {
		s.t.Fatal("resource lookup lost trusted owner or ID")
	}
	if s.cancelRead {
		s.cancel()
	}
}
func (s *resourceBoundaryStore) ListOwnedCollections(c context.Context, o string) ([]domain.CollectionSummary, error) {
	s.read(c, o, "")
	return []domain.CollectionSummary{{ID: "result"}}, s.readErr
}
func (s *resourceBoundaryStore) OwnedCollection(c context.Context, o, id string) (domain.Collection, error) {
	s.read(c, o, id)
	return domain.Collection{ID: "result"}, s.readErr
}
func (s *resourceBoundaryStore) ListOwnedReports(c context.Context, o string) ([]domain.ReportSummary, error) {
	s.read(c, o, "")
	return []domain.ReportSummary{{ID: "result"}}, s.readErr
}
func (s *resourceBoundaryStore) OwnedReport(c context.Context, o, id string) (domain.Report, error) {
	s.read(c, o, id)
	return domain.Report{ReportSummary: domain.ReportSummary{ID: "result"}}, s.readErr
}
func TestResourceCancellationAndDependencies(t *testing.T) {
	for _, method := range []string{"collections list", "collections get", "reports list", "reports get"} {
		for _, scenario := range []string{"success", "nil store", "cancel before", "cancel during read", "auth failure", "store failure"} {
			t.Run(method+"/"+scenario, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				digest := [32]byte{3}
				s := &resourceBoundaryStore{t: t, ctx: ctx, digest: digest, cancel: cancel}
				if method == "collections get" || method == "reports get" {
					s.wantResourceID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
				}
				storeErr := domain.ErrCollectionStore
				if method == "reports list" || method == "reports get" {
					storeErr = domain.ErrReportStore
				}
				want := error(nil)
				authCalls, readCalls := 1, 1
				switch scenario {
				case "nil store":
					want = storeErr
					authCalls = 0
					readCalls = 0
				case "cancel before":
					cancel()
					want = storeErr
					authCalls = 0
					readCalls = 0
				case "cancel during read":
					s.cancelRead = true
					want = storeErr
				case "auth failure":
					s.authErr = domain.ErrSessionStore
					want = domain.ErrSessionStore
					readCalls = 0
				case "store failure":
					s.readErr = storeErr
					want = storeErr
				}
				collections := Collections{Store: s}
				reports := Reports{Store: s}
				if scenario == "nil store" {
					collections.Store = nil
					reports.Store = nil
				}
				var err error
				result := ""
				switch method {
				case "collections list":
					var got []domain.CollectionSummary
					got, err = collections.List(ctx, digest)
					if len(got) > 0 {
						result = got[0].ID
					}
				case "collections get":
					var got domain.Collection
					got, err = collections.Get(ctx, digest, "01ARZ3NDEKTSV4RRFFQ69G5FAV")
					result = got.ID
				case "reports list":
					var got []domain.ReportSummary
					got, err = reports.List(ctx, digest)
					if len(got) > 0 {
						result = got[0].ID
					}
				case "reports get":
					var got domain.Report
					got, err = reports.Get(ctx, digest, "01ARZ3NDEKTSV4RRFFQ69G5FAV")
					result = got.ID
				}
				if err != want || s.authCalls != authCalls || s.readCalls != readCalls {
					t.Fatalf("outcome=%v; auth=%d read=%d", err, s.authCalls, s.readCalls)
				}
				if scenario == "success" && result != "result" {
					t.Fatal("resource lost")
				}
				if scenario == "cancel during read" && result != "" {
					t.Fatal("cancelled response returned resource")
				}
			})
		}
	}
}
