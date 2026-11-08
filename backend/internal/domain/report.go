package domain

import (
	"errors"
	"time"
)

var ErrReportStore = errors.New("report storage operation failed")

type ReportSummary struct {
	ID             string    `json:"id"`
	CollectionName string    `json:"collection_name"`
	ScheduledAt    time.Time `json:"scheduled_at"`
	GeneratedAt    time.Time `json:"generated_at"`
}
type Report struct {
	ReportSummary
	BodyMarkdown     string    `json:"body_markdown"`
	SummaryMarkdown  string    `json:"summary_markdown"`
	UpdatesStartedAt time.Time `json:"updates_started_at"`
	UpdatesEndedAt   time.Time `json:"updates_ended_at"`
}
