package domain

import (
	"strings"
	"testing"
)

func TestNewCollectionValidation(t *testing.T) {
	day := func(n int32) *int32 { return &n }
	for _, tc := range []struct {
		name  string
		value NewCollection
		valid bool
	}{
		{"default", NewCollection{Name: "架空", InitialLookbackMode: "ai"}, true},
		{"max", NewCollection{Name: strings.Repeat("名", 200), ResearchInstruction: strings.Repeat("指", 10000), InitialLookbackMode: "days", InitialLookbackDays: day(365)}, true},
		{"blank", NewCollection{Name: " \n　", InitialLookbackMode: "ai"}, false},
		{"long name", NewCollection{Name: strings.Repeat("名", 201), InitialLookbackMode: "ai"}, false},
		{"long instruction", NewCollection{Name: "x", ResearchInstruction: strings.Repeat("指", 10001), InitialLookbackMode: "ai"}, false},
		{"nul", NewCollection{Name: "x\x00", InitialLookbackMode: "ai"}, false},
		{"utf8", NewCollection{Name: string([]byte{0xff}), InitialLookbackMode: "ai"}, false},
		{"ai days", NewCollection{Name: "x", InitialLookbackMode: "ai", InitialLookbackDays: day(1)}, false},
		{"missing days", NewCollection{Name: "x", InitialLookbackMode: "days"}, false},
		{"zero", NewCollection{Name: "x", InitialLookbackMode: "days", InitialLookbackDays: day(0)}, false},
		{"overflow", NewCollection{Name: "x", InitialLookbackMode: "days", InitialLookbackDays: day(366)}, false},
		{"unknown mode", NewCollection{Name: "x", InitialLookbackMode: "other"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if (tc.value.Validate() == nil) != tc.valid {
				t.Fatal("incorrect validation")
			}
		})
	}
}
