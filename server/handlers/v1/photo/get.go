package photo

import (
	"context"
	"fmt"

	"cameo/internal/ent"
	entPhoto "cameo/internal/ent/photo"
	"cameo/internal/ent/schema_types/photo"
	"cameo/internal/tools/ro"
	"cameo/server/handlers/v1/models"
	"cameo/server/services/request_context"
)

// Get returns one of the couple's uploaded photos.
func (h *Handler) Get(ctx context.Context) (*models.Photo, error) {
	id, err := ro.PathUUID(ctx, "id")
	if err != nil {
		return nil, err
	}

	coupleID, err := coupleID(ctx, h.db)
	if err != nil {
		return nil, err
	}

	row, err := h.db.Photo.Query().
		Where(entPhoto.ID.EQ(id), entPhoto.CoupleID.EQ(coupleID), entPhoto.Status.EQ(photo.StatusUploaded)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load photo: %w", err)
	}

	result, err := models.PresignedPhoto(ctx, h.storage, row, request_context.UserID(ctx), request_context.Time(ctx).Add(downloadExpiry))
	if err != nil {
		return nil, err
	}

	return &result, nil
}
