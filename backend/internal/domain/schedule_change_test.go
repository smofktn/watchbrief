package domain

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSharedCronContract(t *testing.T) {
	raw, err := os.ReadFile("../../../frontend/tests/fixtures/cron.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Expression string
		Valid      bool
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		if got := ValidCron(tc.Expression); got != tc.Valid {
			t.Errorf("cron %q: got %v", tc.Expression, got)
		}
	}
}
