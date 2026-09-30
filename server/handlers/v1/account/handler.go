// Package account handles the authenticated user's own profile.
package account

import "cameo/internal/ent"

type Handler struct {
	db *ent.Client
}

func NewHandler(db *ent.Client) *Handler {
	return &Handler{db: db}
}
