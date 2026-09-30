package photo

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"cameo/internal/ent"
	entPhoto "cameo/internal/ent/photo"
	"cameo/internal/ent/schema_types/photo"
	"cameo/server/handlers/v1/models"
	"cameo/server/services/request_context"
)

type ListRequest struct {
	models.Query
	Favorite bool       `query:"favorite"`
	CallID   *uuid.UUID `query:"call_id"`
}

// List returns the couple's uploaded photos, newest first.
func (h *Handler) List(ctx context.Context, req *ListRequest) (*models.Page[models.Photo], error) {
	coupleID, err := coupleID(ctx, h.db)
	if err != nil {
		return nil, err
	}

	query := h.db.Photo.Query().Where(entPhoto.CoupleID.EQ(coupleID), entPhoto.Status.EQ(photo.StatusUploaded))
	if req.Favorite {
		query.Where(entPhoto.FavoritedBy.Contains(request_context.UserID(ctx)))
	}
	if req.CallID != nil {
		query.Where(entPhoto.CallID.EQ(*req.CallID))
	}

	rows, err := query.Modify(req.Modify).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list photos: %w", err)
	}

	expiresAt := request_context.Time(ctx).Add(downloadExpiry)
	var presignErr error
	page := models.Paginate(rows, req.PageSize(), func(row *ent.Photo) models.Photo {
		item, err := models.PresignedPhoto(ctx, h.storage, row, request_context.UserID(ctx), expiresAt)
		if err != nil {
			presignErr = err
		}
		return item
	}, func(row *ent.Photo) models.Cursor {
		return models.Cursor{Time: row.CreatedAt, ID: row.ID}
	})
	if presignErr != nil {
		return nil, presignErr
	}

	return &page, nil
}
