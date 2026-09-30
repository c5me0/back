package photo

import (
	"context"
	"fmt"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"

	"cameo/internal/ent"
	"cameo/server/services/request_context"
)

type UploadURLRequest struct {
	ContentType        string     `json:"content_type"`
	SizeBytes          int64      `json:"size_bytes"`
	ThumbnailSizeBytes int64      `json:"thumbnail_size_bytes"`
	Width              *int       `json:"width"`
	Height             *int       `json:"height"`
	TakenAt            *time.Time `json:"taken_at"`
}

func (r *UploadURLRequest) Validate() error {
	return validation.ValidateStruct(r,
		validation.Field(&r.ContentType, validation.Required, validation.In("image/jpeg", "image/png", "image/heic", "image/webp")),
		validation.Field(&r.SizeBytes, validation.Required, validation.Min(int64(1)), validation.Max(int64(maxOriginalSize))),
		validation.Field(&r.ThumbnailSizeBytes, validation.Required, validation.Min(int64(1)), validation.Max(int64(maxThumbnailSize))),
		validation.Field(&r.Width, validation.Min(1)),
		validation.Field(&r.Height, validation.Min(1)),
	)
}

type UploadURLResponse struct {
	PhotoID            uuid.UUID `json:"photo_id"`
	UploadURL          string    `json:"upload_url"`
	ThumbnailUploadURL string    `json:"thumbnail_upload_url"`
	ExpiresAt          time.Time `json:"expires_at"`
}

// UploadURL reserves a pending photo and returns presigned PUT URLs for the original and its thumbnail.
func (h *Handler) UploadURL(ctx context.Context, req *UploadURLRequest) (*UploadURLResponse, error) {
	coupleID, err := coupleID(ctx, h.db)
	if err != nil {
		return nil, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate photo id: %w", err)
	}
	prefix := fmt.Sprintf("photos/%s/%s", coupleID, id)

	// InsertBulk instead of Insert: staticcheck v0.8.0 mis-maps facts of builders with generic methods (SA4023 panic).
	rows, err := h.db.Photo.InsertBulk(ent.PhotoInsert{
		ID:           ent.Some(id),
		ContentType:  req.ContentType,
		ObjectKey:    prefix + "/original",
		ThumbnailKey: prefix + "/thumbnail",
		SizeBytes:    req.SizeBytes,
		Width:        ent.FromPtr(req.Width),
		Height:       ent.FromPtr(req.Height),
		TakenAt:      ent.FromPtr(req.TakenAt),
		CoupleID:     coupleID,
		UploaderID:   request_context.UserID(ctx),
	}).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("insert photo: %w", err)
	}

	row := rows[0]

	uploadURL, err := h.storage.PresignPut(ctx, row.ObjectKey, uploadExpiry)
	if err != nil {
		return nil, fmt.Errorf("presign photo upload: %w", err)
	}

	thumbnailUploadURL, err := h.storage.PresignPut(ctx, row.ThumbnailKey, uploadExpiry)
	if err != nil {
		return nil, fmt.Errorf("presign thumbnail upload: %w", err)
	}

	return &UploadURLResponse{
		PhotoID:            row.ID,
		UploadURL:          uploadURL.String(),
		ThumbnailUploadURL: thumbnailUploadURL.String(),
		ExpiresAt:          request_context.Time(ctx).Add(uploadExpiry),
	}, nil
}
