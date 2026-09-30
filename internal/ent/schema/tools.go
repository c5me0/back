package schema

import (
	"github.com/google/uuid"
)

func MustUUIDv7() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}
