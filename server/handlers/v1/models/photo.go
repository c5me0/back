package models

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/google/uuid"

	"cameo/internal/ent"
)

type Photo struct {
	ID           uuid.UUID  `json:"id"`
	UploaderID   uuid.UUID  `json:"uploader_id"`
	CallID       *uuid.UUID `json:"call_id"`
	ContentType  string     `json:"content_type"`
	SizeBytes    int64      `json:"size_bytes"`
	Width        *int       `json:"width"`
	Height       *int       `json:"height"`
	TakenAt      *time.Time `json:"taken_at"`
	IsFavorite   bool       `json:"is_favorite"`
	URL          string     `json:"url"`
	ThumbnailURL string     `json:"thumbnail_url"`
	URLExpiresAt time.Time  `json:"url_expires_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

// Presigner issues download URLs for stored objects.
type Presigner interface {
	PresignGet(ctx context.Context, key string, expiry time.Duration) (*url.URL, error)
}

// PresignedPhoto maps p with download URLs for the original and thumbnail that expire at expiresAt.
func PresignedPhoto(ctx context.Context, presigner Presigner, p *ent.Photo, me uuid.UUID, expiresAt time.Time) (Photo, error) {
	original, err := presigner.PresignGet(ctx, p.ObjectKey, time.Until(expiresAt))
	if err != nil {
		return Photo{}, fmt.Errorf("presign photo: %w", err)
	}

	thumbnail, err := presigner.PresignGet(ctx, p.ThumbnailKey, time.Until(expiresAt))
	if err != nil {
		return Photo{}, fmt.Errorf("presign thumbnail: %w", err)
	}

	return Photo{
		ID:           p.ID,
		UploaderID:   p.UploaderID,
		CallID:       p.CallID,
		ContentType:  p.ContentType,
		SizeBytes:    p.SizeBytes,
		Width:        p.Width,
		Height:       p.Height,
		TakenAt:      p.TakenAt,
		IsFavorite:   slices.Contains(p.FavoritedBy, me),
		URL:          original.String(),
		ThumbnailURL: thumbnail.String(),
		URLExpiresAt: expiresAt,
		CreatedAt:    p.CreatedAt,
	}, nil
}
