package call

import (
	"context"
	"fmt"
	"time"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	entCallHighlight "cameo/internal/ent/callhighlight"
	entPhoto "cameo/internal/ent/photo"
	"cameo/internal/ent/schema_types/call"
	"cameo/internal/ent/schema_types/photo"
	"cameo/internal/tools/ro"
	"cameo/server/handlers/v1/models"
	"cameo/server/services/request_context"
)

// Get returns a call of the couple with its highlights, transcript, recording and shared photos.
func (h *Handler) Get(ctx context.Context) (*models.CallDetail, error) {
	id, err := ro.PathUUID(ctx, "id")
	if err != nil {
		return nil, err
	}

	user, err := currentUser(ctx, h.db)
	if err != nil {
		return nil, err
	}

	row, err := h.db.Call.Query().
		Where(entCall.ID.EQ(id), entCall.CoupleID.EQ(*user.CoupleID)).
		WithHighlights(func(q *ent.CallHighlightQuery) {
			q.Order(entCallHighlight.OffsetSeconds.Asc())
		}).
		WithPhotos(func(q *ent.PhotoQuery) {
			q.Where(entPhoto.Status.EQ(photo.StatusUploaded)).Order(entPhoto.CreatedAt.Asc())
		}).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load call: %w", err)
	}

	expiresAt := request_context.Time(ctx).Add(downloadExpiry)
	detail := models.CallDetail{
		Call:         models.FromCall(row, user.ID, len(row.Edges.Highlights)),
		Highlights:   make([]models.Highlight, 0, len(row.Edges.Highlights)),
		Transcript:   row.Transcript,
		URLExpiresAt: &expiresAt,
		Photos:       make([]models.Photo, 0, len(row.Edges.Photos)),
		ICEServers:   h.iceServers,
	}
	if detail.Transcript == nil {
		detail.Transcript = []call.Segment{}
	}
	for _, highlight := range row.Edges.Highlights {
		detail.Highlights = append(detail.Highlights, models.FromHighlight(highlight))
	}

	if row.RecordingKey != nil {
		url, err := h.storage.PresignGet(ctx, *row.RecordingKey, time.Until(expiresAt))
		if err != nil {
			return nil, fmt.Errorf("presign recording: %w", err)
		}
		detail.RecordingURL = new(url.String())
	}

	for _, shared := range row.Edges.Photos {
		item, err := models.PresignedPhoto(ctx, h.storage, shared, user.ID, expiresAt)
		if err != nil {
			return nil, err
		}
		detail.Photos = append(detail.Photos, item)
	}

	return &detail, nil
}
