package photo

import (
	"context"
	"fmt"

	entPhoto "cameo/internal/ent/photo"
	"cameo/internal/tools/ro"
	"cameo/server/services/request_context"
)

// Delete removes a photo of the couple and its stored objects.
func (h *Handler) Delete(ctx context.Context) error {
	id, err := ro.PathUUID(ctx, "id")
	if err != nil {
		return err
	}

	coupleID, err := coupleID(ctx, h.db)
	if err != nil {
		return err
	}

	rows, err := h.db.Photo.Delete().
		Where(entPhoto.ID.EQ(id), entPhoto.CoupleID.EQ(coupleID)).
		Returning(ctx)
	if err != nil {
		return fmt.Errorf("delete photo: %w", err)
	}
	if len(rows) == 0 {
		return errNotFound
	}

	removeCtx, logger := context.WithoutCancel(ctx), request_context.Logger(ctx)
	for _, key := range []string{rows[0].ObjectKey, rows[0].ThumbnailKey} {
		if err = h.storage.Remove(removeCtx, key); err != nil {
			logger.Warn().Err(err).Str("key", key).Msg("failed to remove photo object")
		}
	}

	return nil
}
