// Package device handles push device registration.
package device

import "cameo/internal/ent"

type Handler struct {
	db *ent.Client
}

func NewHandler(db *ent.Client) *Handler {
	return &Handler{db: db}
}
