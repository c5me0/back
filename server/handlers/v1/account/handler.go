// Package account handles the authenticated user's own profile.
package account

import (
	"cameo/internal/ent"
	"cameo/server/services/purchase"
)

type Handler struct {
	db       *ent.Client
	purchase *purchase.Service
}

func NewHandler(db *ent.Client, purchase *purchase.Service) *Handler {
	return &Handler{db: db, purchase: purchase}
}
