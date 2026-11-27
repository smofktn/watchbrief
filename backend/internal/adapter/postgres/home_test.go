package postgres

import (
	"testing"

	"github.com/smofktn/watchbrief/backend/internal/adapter/postgres/generated"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func TestHomeCollectionsDecodeNoReportAsNullAndValidateIdentifiers(t *testing.T) {
	collectionID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	reportID := "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	rows := []generated.ListHomeCollectionsRow{
		{ID: collectionID, Name: "current name"},
		{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAX", Name: "renamed", LatestReportID: reportID},
	}
	values, err := homeCollections(rows)
	if err != nil || len(values) != 2 || values[0].LatestReportID != nil || values[1].LatestReportID == nil || *values[1].LatestReportID != reportID {
		t.Fatal("home rows did not preserve empty and saved Report references")
	}
	if empty, err := homeCollections(nil); err != nil || empty == nil || len(empty) != 0 {
		t.Fatal("empty home must be represented by a non-nil array")
	}
	if _, err := homeCollections([]generated.ListHomeCollectionsRow{{ID: "bad", Name: "current"}}); err != domain.ErrHomeStore {
		t.Fatal("invalid Collection identifiers must fail closed")
	}
	if _, err := homeCollections([]generated.ListHomeCollectionsRow{{ID: collectionID, Name: "current", LatestReportID: "bad"}}); err != domain.ErrHomeStore {
		t.Fatal("invalid Report identifiers must fail closed")
	}
}
