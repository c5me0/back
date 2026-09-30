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

	result := models.FromUser(me, partner)
	return &result, nil
}
