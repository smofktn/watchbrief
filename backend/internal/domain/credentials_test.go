package domain

import (
	"strings"
	"testing"
)

func TestLoginInputBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, id, password, normalized string
		valid                          bool
	}{
		{"ASCII normalization", "User_1-A", "  long Password  ", "user_1-a", true},
		{"Unicode lower boundary", "user", strings.Repeat("あ", 15), "user", true},
		{"Unicode upper boundary", strings.Repeat("a", 64), strings.Repeat("😀", 128), strings.Repeat("a", 64), true},
		{"short password", "user", strings.Repeat("あ", 14), "", false},
		{"long password", "user", strings.Repeat("😀", 129), "", false},
		{"empty ID", "", strings.Repeat("a", 15), "", false},
		{"long ID", strings.Repeat("a", 65), strings.Repeat("a", 15), "", false},
		{"ID whitespace", " user", strings.Repeat("a", 15), "", false},
		{"non-ASCII ID", "K", strings.Repeat("a", 15), "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateLoginInput(tc.id, tc.password)
			if (err == nil) != tc.valid || got != tc.normalized {
				t.Fatal("unexpected input validation result")
			}
		})
	}
}
