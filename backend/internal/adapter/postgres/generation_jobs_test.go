package postgres

import (
	"github.com/jackc/pgx/v5/pgtype"
	"testing"
	"time"
)

func TestJobSummaryFailsClosedOnInconsistentReport(t *testing.T) {
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	at := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	for _, status := range []string{"queued", "running", "succeeded", "failed", "unknown"} {
		for _, hasReport := range []bool{true, false} {
			report := pgtype.Text{String: id, Valid: hasReport}
			value, err := jobSummary(id, "history", status, at, report)
			valid := status != "unknown" && (status == "succeeded") == hasReport
			if (err == nil) != valid {
				t.Fatal("inconsistent report exposed")
			}
			if valid && (value.ReportID != nil) != hasReport {
				t.Fatal("report mismatch")
			}
		}
	}
}

func TestJobSummaryRejectsUnrepresentableTime(t *testing.T) {
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for _, at := range []pgtype.Timestamptz{{}, {Valid: true, InfinityModifier: pgtype.Infinity}, {Valid: true, Time: time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)}, {Valid: true, Time: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}} {
		if _, err := jobSummary(id, "history", "queued", at, pgtype.Text{}); err == nil {
			t.Fatal("unrepresentable timestamp exposed")
		}
	}
}
