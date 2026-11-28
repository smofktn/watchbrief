package domain

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

const GenerationJobPageSize = 30
const jobCursorTime = "2006-01-02T15:04:05.000000Z"

var ErrGenerationJobCursor = errors.New("invalid job cursor or query")

type GenerationJobCursor struct {
	ScheduledAt time.Time
	ID          string
}
type GenerationJobPage struct {
	GenerationJobs []GenerationJobSummary `json:"generation_jobs"`
	NextCursor     *string                `json:"next_cursor"`
}

func EncodeGenerationJobCursor(row GenerationJobSummary) string {
	return base64.RawURLEncoding.EncodeToString([]byte("job1|" + row.ScheduledAt.UTC().Format(jobCursorTime) + "|" + row.ID))
}
func DecodeGenerationJobCursor(value string) (*GenerationJobCursor, error) {
	if len(value) != 79 {
		return nil, ErrGenerationJobCursor
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(raw) != value {
		return nil, ErrGenerationJobCursor
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 || parts[0] != "job1" || !ValidIdentifier(parts[2]) {
		return nil, ErrGenerationJobCursor
	}
	at, err := time.Parse(jobCursorTime, parts[1])
	if err != nil || at.Year() < 1 || at.Format(jobCursorTime) != parts[1] {
		return nil, ErrGenerationJobCursor
	}
	return &GenerationJobCursor{ScheduledAt: at, ID: parts[2]}, nil
}

var ErrGenerationJobStore = errors.New("generation job storage operation failed")

type GenerationJobSummary struct {
	ID             string    `json:"id"`
	CollectionName string    `json:"collection_name"`
	ScheduledAt    time.Time `json:"scheduled_at"`
	Status         string    `json:"status"`
	ReportID       *string   `json:"report_id"`
}
