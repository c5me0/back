package account

import (
	"context"
	"fmt"

	"cameo/internal/ent"
	entUser "cameo/internal/ent/user"
	"cameo/server/handlers/v1/models"
	"cameo/server/services/request_context"
)

// Me returns the authenticated user.
func (h *Handler) Me(ctx context.Context) (*models.User, error) {
	me, err := h.db.User.Get(ctx, request_context.UserID(ctx))
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}

	return h.user(ctx, me)
}

// user builds the response for me with its partner and the couple's purchase status.
func (h *Handler) user(ctx context.Context, me *ent.User) (*models.User, error) {
	status, err := h.purchase.Status(ctx, me)
	if err != nil {
		return nil, err
	}

	var partner *ent.User
	if me.CoupleID != nil {
		partner, err = h.db.User.Query().
			Where(entUser.CoupleID.EQ(*me.CoupleID), entUser.ID.NEQ(me.ID)).
			Columns(entUser.ID, entUser.DisplayName).
			Only(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return nil, fmt.Errorf("load partner: %w", err)
		}
	}

	result := models.FromUser(me, partner, status)
	return &result, nil
}
