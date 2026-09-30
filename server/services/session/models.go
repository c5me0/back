package session

import "github.com/google/uuid"

// Session is the signed token payload.
type Session struct {
	Version int `cbor:"0,keyasint"`

	ID     uuid.UUID `cbor:"16,keyasint"`
	UserID uuid.UUID `cbor:"17,keyasint"`
}

const tokenVersion = 2
