package domain

import "errors"

var (
	ErrHomeRequest = errors.New("invalid home request")
	ErrHomeStore   = errors.New("home storage operation failed")
)

type HomeCollection struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	LatestReportID *string `json:"latest_report_id"`
}

type Home struct {
	Collections []HomeCollection `json:"collections"`
}
