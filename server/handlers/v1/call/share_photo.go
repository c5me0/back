package call

import (
	"context"
	"fmt"
	"slices"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	entPhoto "cameo/internal/ent/photo"
	"cameo/internal/ent/schema_types/call"
	"cameo/internal/ent/schema_types/photo"
	"cameo/internal/protocol"
	"cameo/internal/tools/ro"
	"cameo/server/handlers/v1/models"
	"cameo/server/services/request_context"
)

type SharePhotoRequest struct {
	PhotoID *uuid.UUID `json:"photo_id"`
}

func (r *SharePhotoRequest) Validate() error {
	return validation.ValidateStruct(r,
		validation.Field(&r.PhotoID, validation.NotNil),
	)
}

// SharePhoto shows one of the couple's photos to the partner during an active call.
func (h *Handler) SharePhoto(ctx context.Context, req *SharePhotoRequest) (*models.Photo, error) {
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
		Columns(entCall.ID, entCall.Status, entCall.CallerID, entCall.CalleeID).
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

	shared, err := h.db.Photo.Query().
		Where(entPhoto.ID.EQ(*req.PhotoID), entPhoto.CoupleID.EQ(*user.CoupleID)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, protocol.ErrorResponse{Code: protocol.NotFound, Message: "photo not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("load photo: %w", err)
	}
	if shared.Status != photo.StatusUploaded {
		return nil, protocol.ErrorResponse{Code: protocol.PhotoInvalidState, Message: "photo is not uploaded"}
	}

	if shared.CallID == nil {
		if err = h.db.Photo.UpdateOneID(shared.ID).Apply(ent.PhotoPatch{CallID: ent.Some(row.ID)}).Exec(ctx); err != nil {
			return nil, fmt.Errorf("attach photo to call: %w", err)
		}
		shared.CallID = &row.ID
	}

	result, err := models.PresignedPhoto(ctx, h.storage, shared, user.ID, request_context.Time(ctx).Add(downloadExpiry))
	if err != nil {
		return nil, err
	}

	peerID := row.CallerID
	if peerID == user.ID {
		peerID = row.CalleeID
	}
	forPeer := result
	forPeer.IsFavorite = slices.Contains(shared.FavoritedBy, peerID)
	h.call.NotifyPeer(row.ID, user.ID, models.NewPhotoShared(forPeer))

	title := ""
	if user.DisplayName != nil {
		title = *user.DisplayName
	}
	//nolint:contextcheck // push delivery runs in the background, detached from ctx
	h.push.NotifyUser(peerID, false, title, "사진을 공유했어요", map[string]any{
		"type":     "photo_shared",
		"call_id":  row.ID,
		"photo_id": shared.ID,
	})

	return &result, nil
}
