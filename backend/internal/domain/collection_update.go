package domain

import (
	"errors"
	"fmt"
	"strconv"
)

var ErrUpdateConflict = errors.New("collection has changed")
var ErrUpdateRequestConflict = errors.New("update request conflicts")
var ErrCollectionNotStopped = errors.New("collection must be stopped")

type CollectionUpdate struct {
	NewCollection
	Version string         `json:"version"`
	Targets []TargetChange `json:"targets,omitempty"`
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
	if err := input.NewCollection.Validate(); err != nil {
		return err
	}
	if len(input.Targets) > 20 {
		return &CollectionInputError{Fields: map[string]string{"targets": "1回に保存できる対象変更は20行までです。"}}
	}
	seen := map[string]bool{}
	for i, target := range input.Targets {
		if target.ID != nil {
			if !ValidIdentifier(*target.ID) || seen[*target.ID] {
				return ErrInvalidIdentifier
			}
			seen[*target.ID] = true
		}
		// Share the established Unicode/whitespace/length rules with Collection fields.
		text := NewCollection{Name: target.Name, ResearchInstruction: target.Instruction, InitialLookbackMode: "ai"}
		if err := text.Validate(); err != nil {
			return &CollectionInputError{Fields: map[string]string{fmt.Sprintf("targets.%d", i): "対象名は1〜200文字、補足指示は10,000文字以内で入力してください。"}}
		}
	}
	return nil
}

type CollectionUpdateResult struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// TargetChange is an explicit edit, never a replacement/deletion of omitted rows.
type TargetChange struct {
	ID          *string `json:"id"`
	Name        string  `json:"name"`
	Instruction string  `json:"instruction"`
}
