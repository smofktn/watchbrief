package identifier

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

func NewULID() (string, error) {
	id, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
	return id.String(), err
}
