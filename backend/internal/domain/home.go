package domain

import "errors"

var (
	ErrHomeRequest = errors.New("invalid home request")
	ErrHomeStore   = errors.New("home storage operation failed")
)

type HomeCollection struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Version        string        `json:"version"`
	LatestReportID *string       `json:"latest_report_id"`
	TodayJobCounts HomeJobCounts `json:"today_job_counts"`
}

type HomeJobCounts struct {
	Queued    int64 `json:"queued"`
	Running   int64 `json:"running"`
	Succeeded int64 `json:"succeeded"`
	Failed    int64 `json:"failed"`
}

type Home struct {
	AsOfDate    string           `json:"as_of_date"`
	Collections []HomeCollection `json:"collections"`
}
