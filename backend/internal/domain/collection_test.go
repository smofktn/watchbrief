package domain

import (
	"strings"
	"testing"
)

func TestResourceIdentifierBoundary(t *testing.T) {
	valid := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for _, id := range []string{valid, "00000000000000000000000000", "7ZZZZZZZZZZZZZZZZZZZZZZZZZ"} {
		if !ValidIdentifier(id) {
			t.Fatal("canonical identifier rejected")
		}
	}
	for _, id := range []string{"", valid[:25], valid + "0", "8" + valid[1:], strings.ToLower(valid), "I" + valid[1:], valid[:25] + "O", valid[:25] + "L", valid[:25] + "U", valid + "\n", " " + valid} {
		if ValidIdentifier(id) {
			t.Fatal("noncanonical identifier accepted")
		}
	}
}
