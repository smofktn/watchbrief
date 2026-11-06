package domain

import (
	"strings"
	"testing"
)

func TestPasswordNFC(t *testing.T) {
	for _, tc := range []struct {
		name, input, expected string
		valid                 bool
	}{
		{"compose before minimum", strings.Repeat("か\u3099", 14), "", false},
		{"minimum", strings.Repeat("か\u3099", 15), strings.Repeat("が", 15), true},
		{"maximum decomposed", strings.Repeat("か\u3099", 128), strings.Repeat("が", 128), true},
		{"above maximum", strings.Repeat("か\u3099", 129), "", false},
		{"preserve case spaces and width", "  AaＡａ password  ", "  AaＡａ password  ", true},
		{"invalid UTF8", "example password \xff", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizePassword(tc.input)
			if (err == nil) != tc.valid || got != tc.expected {
				t.Fatal("unexpected NFC/password validation result")
			}
		})
	}
}
