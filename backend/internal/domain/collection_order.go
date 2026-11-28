package domain

import (
	"errors"
)

var (
	ErrCollectionOrderInput           = errors.New("invalid collection order input")
	ErrCollectionOrderConflict        = errors.New("collection order has changed")
	ErrCollectionOrderRequestConflict = errors.New("collection order request conflicts")
)

type CollectionOrderEntry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type CollectionOrderInput struct {
	Expected   []CollectionOrderEntry `json:"expected"`
	OrderedIDs []string               `json:"ordered_ids"`
}

func (input CollectionOrderInput) Validate() error {
	if len(input.Expected) != len(input.OrderedIDs) {
		return ErrCollectionOrderInput
	}
	expected := make(map[string]struct{}, len(input.Expected))
	for _, entry := range input.Expected {
		if !ValidIdentifier(entry.ID) || !ValidCollectionVersion(entry.Version) {
			return ErrCollectionOrderInput
		}
		if _, exists := expected[entry.ID]; exists {
			return ErrCollectionOrderInput
		}
		expected[entry.ID] = struct{}{}
	}
	ordered := make(map[string]struct{}, len(input.OrderedIDs))
	for _, id := range input.OrderedIDs {
		if !ValidIdentifier(id) {
			return ErrCollectionOrderInput
		}
		if _, exists := ordered[id]; exists {
			return ErrCollectionOrderInput
		}
		ordered[id] = struct{}{}
	}
	for id := range expected {
		if _, exists := ordered[id]; !exists {
			return ErrCollectionOrderInput
		}
	}
	return nil
}
