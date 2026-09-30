package photo

import (
	"context"
	"fmt"
	"slices"

	"cameo/internal/ent"
	entPhoto "cameo/internal/ent/photo"
	"cameo/internal/ent/schema_types/photo"
	"cameo/internal/tools/ro"
	"cameo/server/services/request_context"
)

// Favorite adds the photo to the user's favorites.
func (h *Handler) Favorite(ctx context.Context) (ret error) {
	id, err := ro.PathUUID(ctx, "id")
	if err != nil {
		return err
	}

	tx, commit, err := request_context.Tx(ctx, h.db)
	if err != nil {
		return err
	}
	defer commit(&ret)

	coupleID, err := coupleID(ctx, tx.Client())
	if err != nil {
		return err
	}

	row, err := tx.Photo.Query().
		Where(entPhoto.ID.EQ(id), entPhoto.CoupleID.EQ(coupleID), entPhoto.Status.EQ(photo.StatusUploaded)).
		Columns(entPhoto.ID, entPhoto.FavoritedBy).
		ForUpdate().
		Only(ctx)
	if ent.IsNotFound(err) {
		return errNotFound
	}
	if err != nil {
		return fmt.Errorf("load photo: %w", err)
	}

	me := request_context.UserID(ctx)
	if slices.Contains(row.FavoritedBy, me) {
		return nil
	}

	if err = tx.Photo.UpdateOneID(row.ID).Apply(ent.PhotoPatch{FavoritedBy: ent.Some(append(row.FavoritedBy, me))}).Exec(ctx); err != nil {
		return fmt.Errorf("update photo: %w", err)
	}

	return nil
}
