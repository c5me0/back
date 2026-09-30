package device

import (
	"context"
	"fmt"

	"github.com/go-chi/chi/v5"

	entDevice "cameo/internal/ent/device"
	"cameo/internal/protocol"
	"cameo/server/services/request_context"
)

// Delete unregisters one of the user's push tokens.
func (h *Handler) Delete(ctx context.Context) error {
	deleted, err := h.db.Device.Delete().
		Where(entDevice.Token.EQ(chi.URLParamFromCtx(ctx, "token")), entDevice.UserID.EQ(request_context.UserID(ctx))).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete device: %w", err)
	}
	if deleted == 0 {
		return protocol.ErrorResponse{Code: protocol.NotFound, Message: "device not found"}
	}

	return nil
}
