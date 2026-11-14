package domain

import (
	"errors"
	"strconv"
)

var ErrUpdateConflict = errors.New("collection has changed")
var ErrUpdateRequestConflict = errors.New("update request conflicts")
var ErrCollectionNotStopped = errors.New("collection must be stopped")

type CollectionUpdate struct {
	NewCollection
	Version string `json:"version"`
}

// Decimal strings avoid loss of bigint precision in browser JSON.
func ValidCollectionVersion(value string) bool {
	n, err := strconv.ParseInt(value, 10, 64)
	return err == nil && n > 0 && strconv.FormatInt(n, 10) == value
}

func (input CollectionUpdate) Validate() error {
	if !ValidCollectionVersion(input.Version) {
		return ErrInvalidIdentifier
	}
	return input.NewCollection.Validate()
}

type CollectionUpdateResult struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}
