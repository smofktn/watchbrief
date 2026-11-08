package identifier

import (
	"github.com/oklog/ulid/v2"
	"testing"
	"time"
)

func TestCanonicalFreshULIDs(t *testing.T) {
	before := ulid.Timestamp(time.Now())
	seen := map[string]bool{}
	for range 32 {
		id, err := NewULID()
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ulid.ParseStrict(id)
		if err != nil || parsed.String() != id || len(id) != 26 || seen[id] {
			t.Fatal("identifier is not a fresh canonical ULID")
		}
		seen[id] = true
		if parsed.Time() < before || parsed.Time() > ulid.Timestamp(time.Now()) {
			t.Fatal("identifier time outside creation interval")
		}
	}
}
