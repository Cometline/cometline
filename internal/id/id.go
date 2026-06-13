package id

import (
	"github.com/oklog/ulid/v2"
)

// New returns a time-ordered ULID string suitable for primary keys.
func New() string {
	return ulid.Make().String()
}
