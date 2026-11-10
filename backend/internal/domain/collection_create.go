package domain

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ErrCreationConflict = errors.New("creation request conflicts")
var creationKeyPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func ValidCreationKey(key string) bool { return creationKeyPattern.MatchString(key) }

type NewCollection struct {
	Name                string `json:"name"`
	ResearchInstruction string `json:"research_instruction"`
	InitialLookbackMode string `json:"initial_lookback_mode"`
	InitialLookbackDays *int32 `json:"initial_lookback_days"`
}

type CollectionInputError struct{ Fields map[string]string }

func (e *CollectionInputError) Error() string { return "invalid collection input" }

func (input NewCollection) Validate() error {
	fields := map[string]string{}
	validText := func(s string) bool { return utf8.ValidString(s) && !strings.ContainsRune(s, 0) }
	if !validText(input.Name) || strings.TrimSpace(input.Name) == "" || utf8.RuneCountInString(input.Name) > 200 {
		fields["name"] = "名前は空白だけにせず、1〜200文字で入力してください。"
	}
	if !validText(input.ResearchInstruction) || utf8.RuneCountInString(input.ResearchInstruction) > 10000 {
		fields["research_instruction"] = "調査方針は10,000文字以内で入力してください。"
	}
	if !(input.InitialLookbackMode == "ai" && input.InitialLookbackDays == nil ||
		input.InitialLookbackMode == "days" && input.InitialLookbackDays != nil && *input.InitialLookbackDays >= 1 && *input.InitialLookbackDays <= 365) {
		fields["initial_lookback_days"] = "初回の更新確認はAIにおまかせ、または1〜365日を指定してください。"
	}
	if len(fields) > 0 {
		return &CollectionInputError{Fields: fields}
	}
	return nil
}
