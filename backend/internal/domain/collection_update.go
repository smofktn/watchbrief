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
	Sources []SourceChange `json:"sources,omitempty"`
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
	if len(input.Sources) > 20 {
		return &CollectionInputError{Fields: map[string]string{"sources": "1回に保存できる情報源変更は20行までです。"}}
	}
	sourceIDs := map[string]bool{}
	for _, source := range input.Sources {
		if !ValidIdentifier(source.WatchTargetID) {
			return ErrInvalidIdentifier
		}
		if source.ID != nil {
			if !ValidIdentifier(*source.ID) || sourceIDs[*source.ID] {
				return ErrInvalidIdentifier
			}
			sourceIDs[*source.ID] = true
		}
		text := NewCollection{Name: source.Name, ResearchInstruction: source.Instruction, InitialLookbackMode: "ai"}
		if text.Validate() != nil || !ValidSourceURL(source.URL) {
			return &CollectionInputError{Fields: map[string]string{"sources": "情報源の表示名・URL・補足指示を確認してください。"}}
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
