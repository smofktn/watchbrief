package domain

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestReportCursorMicrosecondsAndCanonicalForm(t *testing.T) {
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	at := time.Date(2026, 4, 1, 0, 0, 0, 123456000, time.UTC)
	value := EncodeReportCursor(ReportSummary{ID: id, ScheduledAt: at})
	cursor, err := DecodeReportCursor(value)
	if err != nil || cursor.ID != id || !cursor.ScheduledAt.Equal(at) || len(value) != 75 {
		t.Fatal("precision lost")
	}
	for _, raw := range []string{"2|2026-04-01T00:00:00.123456Z|" + id, "1|2026-02-30T00:00:00.123456Z|" + id, "1|2026-04-01T00:00:00.12345Z|" + id, "1|0000-04-01T00:00:00.123456Z|" + id, "1|2026-04-01T00:00:00.123456Z|" + strings.ToLower(id), "1|2026-04-01T00:00:00.123456+00:00|" + id} {
		if _, err := DecodeReportCursor(base64.RawURLEncoding.EncodeToString([]byte(raw))); err == nil {
			t.Fatal("noncanonical cursor accepted")
		}
	}
	for _, bad := range []string{"", value + "=", strings.Repeat("a", 76), value[:74] + "!", value[:74] + "B"} {
		if _, err := DecodeReportCursor(bad); err == nil {
			t.Fatal("invalid base64 cursor accepted")
		}
	}
}
