package domain

import (
	"errors"
	"regexp"
)

var (
	ErrResourceNotFound  = errors.New("resource not found")
	ErrInvalidIdentifier = errors.New("invalid identifier")
	ErrCollectionStore   = errors.New("collection storage operation failed")
	identifierPattern    = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)
)

func ValidIdentifier(value string) bool { return identifierPattern.MatchString(value) }

type CollectionSummary struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	DisplayOrder int32  `json:"display_order"`
}

type Collection struct {
	CollectionSummary
	Version             string              `json:"version"`
	ResearchInstruction string              `json:"research_instruction"`
	InitialLookbackMode string              `json:"initial_lookback_mode"`
	InitialLookbackDays *int32              `json:"initial_lookback_days"`
	Schedule            *CollectionSchedule `json:"schedule"`
	Targets             []WatchTarget       `json:"targets"`
}

type CollectionSchedule struct {
	ID             string `json:"id"`
	CronExpression string `json:"cron_expression"`
}
type WatchTarget struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Instruction  string              `json:"instruction"`
	Enabled      bool                `json:"enabled"`
	DisplayOrder int32               `json:"display_order"`
	Sources      []InformationSource `json:"sources"`
}
type InformationSource struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Instruction string `json:"instruction"`
}
