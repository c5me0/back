package photo

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"cameo/internal/ent"
	entPhoto "cameo/internal/ent/photo"
	"cameo/internal/ent/schema_types/photo"
	"cameo/internal/tools/ro"
	"cameo/server/services/request_context"
)

// Unfavorite removes the photo from the user's favorites.
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
	if !slices.Contains(row.FavoritedBy, me) {
		return nil
	}

	favoritedBy := slices.DeleteFunc(row.FavoritedBy, func(userID uuid.UUID) bool { return userID == me })
	if err = tx.Photo.UpdateOneID(row.ID).Apply(ent.PhotoPatch{FavoritedBy: ent.Some(favoritedBy)}).Exec(ctx); err != nil {
		return fmt.Errorf("update photo: %w", err)
	}

	return nil
}
