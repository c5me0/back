package call

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	"cameo/internal/tools/ro"
	"cameo/server/services/request_context"
)

// Unfavorite removes the call from the user's favorites.
func (h *Handler) Unfavorite(ctx context.Context) (ret error) {
	id, err := ro.PathUUID(ctx, "id")
	if err != nil {
		return err
	}

	tx, commit, err := request_context.Tx(ctx, h.db)
	if err != nil {
		return err
	}
	defer commit(&ret)

	user, err := currentUser(ctx, tx.Client())
	if err != nil {
		return err
	}

	row, err := tx.Call.Query().
		Where(entCall.ID.EQ(id), entCall.CoupleID.EQ(*user.CoupleID)).
		Columns(entCall.ID, entCall.FavoritedBy).
		ForUpdate().
		Only(ctx)
	if ent.IsNotFound(err) {
		return errNotFound
	}
	if err != nil {
		return fmt.Errorf("load call: %w", err)
	}
	if !slices.Contains(row.FavoritedBy, user.ID) {
		return nil
	}

	favoritedBy := slices.DeleteFunc(row.FavoritedBy, func(userID uuid.UUID) bool { return userID == user.ID })
	if err = tx.Call.UpdateOneID(row.ID).Apply(ent.CallPatch{FavoritedBy: ent.Some(favoritedBy)}).Exec(ctx); err != nil {
		return fmt.Errorf("update call: %w", err)
	}

	return nil
}
