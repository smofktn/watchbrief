package postgres

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/smofktn/watchbrief/backend/internal/domain"
)

func TestCollectionJobRowsDistinguishUnavailableAndEmpty(t *testing.T) {
	missing := []collectionJobRow{{exists: false}}
	if exists, rows, err := collectionJobSummaries(missing); err != nil || exists || rows != nil {
		t.Fatal("missing Collection marker was not kept distinct")
	}
	empty := []collectionJobRow{{exists: true}}
	if exists, rows, err := collectionJobSummaries(empty); err != nil || !exists || rows == nil || len(rows) != 0 {
		t.Fatal("empty owned Collection page was not decoded")
	}
	if _, _, err := collectionJobSummaries([]collectionJobRow{{exists: true}, {exists: true, id: pgtype.Text{String: "x", Valid: true}}}); err == nil {
		t.Fatal("malformed sentinel rows must fail closed")
	}
}

func TestCollectionJobRowsValidateEverySummary(t *testing.T) {
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	row := collectionJobRow{
		exists: true, id: pgtype.Text{String: id, Valid: true}, name: pgtype.Text{String: "history", Valid: true},
		at:     pgtype.Timestamptz{Time: time.Date(2026, 11, 27, 0, 0, 0, 1, time.UTC), Valid: true},
		status: pgtype.Text{String: "queued", Valid: true},
	}
	exists, values, err := collectionJobSummaries([]collectionJobRow{row})
	if err != nil || !exists || len(values) != 1 || values[0].ReportID != nil {
		t.Fatal("valid queued job was rejected")
	}
	row.status = pgtype.Text{String: "succeeded", Valid: true}
	if _, _, err := collectionJobSummaries([]collectionJobRow{row}); err == nil {
		t.Fatal("succeeded job without Report must fail closed")
	}
}

func TestCollectionJobRowsRejectInconsistentCollectionExistence(t *testing.T) {
	rows := []collectionJobRow{{exists: true}, {exists: false}}
	if _, _, err := collectionJobSummaries(rows); err == nil || err == domain.ErrResourceNotFound {
		t.Fatal("inconsistent collection existence must fail closed")
	}
	page := collectionJobRow{
		exists: false, id: pgtype.Text{String: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Valid: true},
		name:   pgtype.Text{String: "history", Valid: true},
		at:     pgtype.Timestamptz{Time: time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC), Valid: true},
		status: pgtype.Text{String: "queued", Valid: true},
	}
	if _, _, err := collectionJobSummaries([]collectionJobRow{page}); err == nil {
		t.Fatal("Job rows without an owned Collection must fail closed")
	}
}
