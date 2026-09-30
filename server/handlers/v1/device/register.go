package device

import (
	"context"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"cameo/internal/ent"
	entDevice "cameo/internal/ent/device"
	"cameo/internal/ent/schema_types/device"
	"cameo/server/services/request_context"
)

type RegisterRequest struct {
	Platform *device.Platform `json:"platform"`
	Token    string           `json:"token"`
}

func (r *RegisterRequest) Validate() error {
	return validation.ValidateStruct(r,
		validation.Field(&r.Platform, validation.NotNil),
		validation.Field(&r.Token, validation.Required, validation.Length(1, 200)),
	)
}

// Register binds a push token to the current session, taking it over from any previous owner.
func (h *Handler) Register(ctx context.Context, req *RegisterRequest) error {
	err := h.db.Device.Insert(ent.DeviceInsert{
		Platform:  *req.Platform,
		Token:     req.Token,
		UserID:    request_context.UserID(ctx),
		SessionID: request_context.Session(ctx).ID,
	}).OnConflict(entDevice.Platform, entDevice.Token).UpdateNewValues().Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert device: %w", err)
	}

	return nil
}
