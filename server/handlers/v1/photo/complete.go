package photo

import (
	"context"
	"fmt"

	"github.com/minio/minio-go/v7"

	"cameo/internal/ent"
	entPhoto "cameo/internal/ent/photo"
	"cameo/internal/ent/schema_types/photo"
	"cameo/internal/protocol"
	"cameo/internal/tools/ro"
	"cameo/server/handlers/v1/models"
	"cameo/server/services/request_context"
)

// Complete verifies both uploaded objects and publishes the pending photo.
func (h *Handler) Complete(ctx context.Context) (res *models.Photo, ret error) {
	id, err := ro.PathUUID(ctx, "id")
	if err != nil {
		return nil, err
	}

	tx, commit, err := request_context.Tx(ctx, h.db)
	if err != nil {
		return nil, err
	}
	defer commit(&ret)

	coupleID, err := coupleID(ctx, tx.Client())
	if err != nil {
		return nil, err
	}

	row, err := tx.Photo.Query().
		Where(entPhoto.ID.EQ(id), entPhoto.CoupleID.EQ(coupleID)).
		ForUpdate().
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load photo: %w", err)
	}
	if row.UploaderID != request_context.UserID(ctx) {
		return nil, protocol.ErrorResponse{Code: protocol.Forbidden, Message: "only the uploader can complete the photo"}
	}
	if row.Status != photo.StatusPending {
		return nil, protocol.ErrorResponse{Code: protocol.PhotoInvalidState, Message: "photo is already uploaded"}
	}

	original, err := h.storage.Stat(ctx, row.ObjectKey)
	if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
		return nil, protocol.ErrorResponse{Code: protocol.PhotoUploadIncomplete, Message: "photo is not uploaded"}
	}
	if err != nil {
		return nil, fmt.Errorf("stat photo: %w", err)
	}

	thumbnail, err := h.storage.Stat(ctx, row.ThumbnailKey)
	if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
		return nil, protocol.ErrorResponse{Code: protocol.PhotoUploadIncomplete, Message: "thumbnail is not uploaded"}
	}
	if err != nil {
		return nil, fmt.Errorf("stat thumbnail: %w", err)
	}

	if original.Size > maxOriginalSize || thumbnail.Size > maxThumbnailSize {
		logger := request_context.Logger(ctx)
		for _, key := range []string{row.ObjectKey, row.ThumbnailKey} {
			if err = h.storage.Remove(ctx, key); err != nil {
				logger.Warn().Err(err).Str("key", key).Msg("failed to remove oversized upload")
			}
		}
		return nil, protocol.ErrorResponse{Code: protocol.PhotoUploadIncomplete, Message: "uploaded object exceeds the size limit"}
	}

	// Exec instead of Save: staticcheck v0.8.0 mis-maps facts of builders with generic methods (SA4023 panic).
	err = tx.Photo.UpdateOneID(row.ID).Apply(ent.PhotoPatch{
		Status:    ent.Some(photo.StatusUploaded),
		SizeBytes: ent.Some(original.Size),
	}).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update photo: %w", err)
	}
	row.Status = photo.StatusUploaded
	row.SizeBytes = original.Size

	result, err := models.PresignedPhoto(ctx, h.storage, row, request_context.UserID(ctx), request_context.Time(ctx).Add(downloadExpiry))
	if err != nil {
		return nil, err
	}

	return &result, nil
}
