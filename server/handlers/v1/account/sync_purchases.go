package account

import (
	"context"

	"cameo/server/handlers/v1/models"
	"cameo/server/services/request_context"
)

// SyncPurchases refreshes the authenticated user's purchases from RevenueCat, for clients right after a purchase.
func (h *Handler) SyncPurchases(ctx context.Context) (*models.User, error) {
	me, err := h.purchase.Sync(ctx, request_context.UserID(ctx))
	if err != nil {
		return nil, err
	}

	return h.user(ctx, me)
}
