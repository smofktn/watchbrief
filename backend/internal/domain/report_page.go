package domain

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

const ReportPageSize = 30
const reportCursorTime = "2006-01-02T15:04:05.000000Z"

var ErrReportCursor = errors.New("invalid report cursor or query")

type ReportCursor struct {
	ScheduledAt time.Time
	ID          string
}
type ReportPage struct {
	Reports    []ReportSummary `json:"reports"`
	NextCursor *string         `json:"next_cursor"`
}

func EncodeReportCursor(row ReportSummary) string {
	return base64.RawURLEncoding.EncodeToString([]byte("1|" + row.ScheduledAt.UTC().Format(reportCursorTime) + "|" + row.ID))
}
func DecodeReportCursor(value string) (*ReportCursor, error) {
	if len(value) != 75 {
		return nil, ErrReportCursor
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(raw) != value {
		return nil, ErrReportCursor
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 || parts[0] != "1" || !ValidIdentifier(parts[2]) {
		return nil, ErrReportCursor
	}
	at, err := time.Parse(reportCursorTime, parts[1])
	if err != nil || at.Year() < 1 || at.Format(reportCursorTime) != parts[1] {
		return nil, ErrReportCursor
	}
	return &ReportCursor{ScheduledAt: at, ID: parts[2]}, nil
}
