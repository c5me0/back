package call

import (
	"context"
	"fmt"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	"cameo/internal/tools/ro"
)

// Decline rejects a ringing call as its callee.
func (h *Handler) Decline(ctx context.Context) error {
	id, err := ro.PathUUID(ctx, "id")
	if err != nil {
		return err
	}

	user, err := currentUser(ctx, h.db)
	if err != nil {
		return err
	}

	row, err := h.db.Call.Query().
		Where(entCall.ID.EQ(id), entCall.CoupleID.EQ(*user.CoupleID)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return errNotFound
	}
	if err != nil {
		return fmt.Errorf("load call: %w", err)
	}

	return h.call.Decline(ctx, row, user.ID)
}
