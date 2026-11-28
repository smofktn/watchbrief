package domain

import (
	"errors"
	"testing"
)

func TestCollectionOrderValidation(t *testing.T) {
	one := CollectionOrderInput{
		Expected:   []CollectionOrderEntry{{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Version: "9223372036854775807"}},
		OrderedIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"},
	}
	if err := one.Validate(); err != nil {
		t.Fatal("valid one-row order rejected")
	}
	if err := (CollectionOrderInput{}).Validate(); err != nil {
		t.Fatal("empty order rejected")
	}
	for name, input := range map[string]CollectionOrderInput{
		"duplicate expected": {Expected: []CollectionOrderEntry{{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Version: "1"}, {ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Version: "1"}}, OrderedIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV", "01ARZ3NDEKTSV4RRFFQ69G5FAV"}},
		"duplicate desired":  {Expected: []CollectionOrderEntry{{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Version: "1"}, {ID: "01ARZ3NDEKTSV4RRFFQ69G5FAW", Version: "1"}}, OrderedIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV", "01ARZ3NDEKTSV4RRFFQ69G5FAV"}},
		"different set":      {Expected: []CollectionOrderEntry{{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Version: "1"}}, OrderedIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAW"}},
		"invalid version":    {Expected: []CollectionOrderEntry{{ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Version: "092"}}, OrderedIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}},
	} {
		t.Run(name, func(t *testing.T) {
			if !errors.Is(input.Validate(), ErrCollectionOrderInput) {
				t.Fatal("invalid order accepted")
			}
		})
	}
}
