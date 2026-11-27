package domain

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestGenerationJobCursorMicrosecondsAndCanonicalForm(t *testing.T) {
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	at := time.Date(2026, 4, 1, 0, 0, 0, 123456000, time.UTC)
	value := EncodeGenerationJobCursor(GenerationJobSummary{ID: id, ScheduledAt: at})
	cursor, err := DecodeGenerationJobCursor(value)
	if err != nil || cursor.ID != id || !cursor.ScheduledAt.Equal(at) || len(value) != 79 {
		t.Fatal("precision lost")
	}
	for _, raw := range []string{"job2|2026-04-01T00:00:00.123456Z|" + id, "job1|2026-02-30T00:00:00.123456Z|" + id, "job1|2026-04-01T00:00:00.12345Z|" + id, "job1|0000-04-01T00:00:00.123456Z|" + id, "job1|2026-04-01T00:00:00.123456Z|" + strings.ToLower(id), "job1|2026-04-01T00:00:00.123456+00:00|" + id} {
		if _, err := DecodeGenerationJobCursor(base64.RawURLEncoding.EncodeToString([]byte(raw))); err == nil {
			t.Fatal("noncanonical cursor accepted")
		}
	}
	for _, bad := range []string{"", value + "=", strings.Repeat("a", 80), value[:78] + "!", value[:78] + "B"} {
		if _, err := DecodeGenerationJobCursor(bad); err == nil {
			t.Fatal("invalid base64 cursor accepted")
		}
	}
}

func TestCursorTypesCannotBeConfused(t *testing.T) {
	at := time.Now()
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	if _, err := DecodeReportCursor(EncodeGenerationJobCursor(GenerationJobSummary{ID: id, ScheduledAt: at})); err == nil {
		t.Fatal("job cursor accepted by reports")
	}
	if _, err := DecodeGenerationJobCursor(EncodeReportCursor(ReportSummary{ID: id, ScheduledAt: at})); err == nil {
		t.Fatal("report cursor accepted by jobs")
	}
}
