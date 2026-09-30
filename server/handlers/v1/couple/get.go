package couple

import (
	"context"
	"fmt"

	entUser "cameo/internal/ent/user"
	"cameo/server/handlers/v1/models"
	"cameo/server/services/request_context"
)

// Get returns the authenticated user's couple.
func (h *Handler) Get(ctx context.Context) (*models.Couple, error) {
	me, err := h.db.User.Query().
		Where(entUser.ID.EQ(request_context.UserID(ctx))).
		Columns(entUser.ID, entUser.CoupleID).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}
	if me.CoupleID == nil {
		return nil, errNotConnected
	}

	couple, err := h.db.Couple.Get(ctx, *me.CoupleID)
	if err != nil {
		return nil, fmt.Errorf("load couple: %w", err)
	}

	partner, err := h.db.User.Query().
		Where(entUser.CoupleID.EQ(couple.ID), entUser.ID.NEQ(me.ID)).
		Columns(entUser.ID, entUser.DisplayName, entUser.Phone).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("load partner: %w", err)
	}

	restorable, err := restorable(ctx, h.db, couple)
	if err != nil {
		return nil, err
	}

	result := models.FromCouple(couple, partner, restorable)
	return &result, nil
}
