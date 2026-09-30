package call

import (
	"context"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	"cameo/internal/ent/schema_types/call"
	"cameo/internal/tools/ro"
	"cameo/server/handlers/v1/models"
)

type CreateHighlightRequest struct {
	OffsetSeconds *float64 `json:"offset_seconds"`
}

func (r *CreateHighlightRequest) Validate() error {
	return validation.ValidateStruct(r,
		validation.Field(&r.OffsetSeconds, validation.NotNil, validation.Min(0.0)),
	)
}

// CreateHighlight marks a moment of an active call.
func (h *Handler) CreateHighlight(ctx context.Context, req *CreateHighlightRequest) (*models.Highlight, error) {
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
		Columns(entCall.ID, entCall.Status).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load call: %w", err)
	}
	if row.Status != call.StatusActive {
		return nil, errNotActive
	}

	// InsertBulk instead of Insert: staticcheck v0.8.0 mis-maps facts of builders with generic methods (SA4023 panic).
	highlights, err := h.db.CallHighlight.InsertBulk(ent.CallHighlightInsert{
		OffsetSeconds: *req.OffsetSeconds,
		CallID:        row.ID,
		UserID:        user.ID,
	}).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("insert highlight: %w", err)
	}

	result := models.FromHighlight(highlights[0])
	h.call.NotifyPeer(row.ID, user.ID, models.NewHighlightAdded(result))
	return &result, nil
}
