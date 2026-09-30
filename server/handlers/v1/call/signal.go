package call

import (
	"fmt"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	"cameo/internal/ent/schema_types/call"
	"cameo/internal/protocol"
	"cameo/internal/tools/ro"
	"cameo/server/services/request_context"
)

const signalReadLimit = 64 << 10

// Signal upgrades to the call's signaling WebSocket and serves it until it closes.
func (h *Handler) Signal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := request_context.Logger(ctx)

	id, err := ro.PathUUID(ctx, "id")
	if err != nil {
		ro.WriteError(w, r, err)
		return
	}

	user, err := currentUser(ctx, h.db)
	if err != nil {
		ro.WriteError(w, r, err)
		return
	}

	row, err := h.db.Call.Query().
		Where(entCall.ID.EQ(id), entCall.CoupleID.EQ(*user.CoupleID)).
		Columns(entCall.ID, entCall.Status).
		Only(ctx)
	if ent.IsNotFound(err) {
		ro.WriteError(w, r, errNotFound)
		return
	}
	if err != nil {
		ro.WriteError(w, r, fmt.Errorf("load call: %w", err))
		return
	}
	if (row.Status != call.StatusRinging && row.Status != call.StatusActive) || !h.call.HasRoom(row.ID) {
		ro.WriteError(w, r, protocol.ErrorResponse{Code: protocol.CallInvalidState, Message: "call is not live"})
		return
	}

	// The socket outlives the server's read and write timeouts.
	controller := http.NewResponseController(w)
	if err = controller.SetReadDeadline(time.Time{}); err != nil {
		ro.WriteError(w, r, fmt.Errorf("clear read deadline: %w", err))
		return
	}
	if err = controller.SetWriteDeadline(time.Time{}); err != nil {
		ro.WriteError(w, r, fmt.Errorf("clear write deadline: %w", err))
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		logger.Debug().Err(err).Msg("failed to accept signaling socket")
		return
	}
	conn.SetReadLimit(signalReadLimit)

	if err = h.call.Join(ctx, row.ID, user.ID, conn); err != nil {
		logger.Debug().Err(err).Msg("signaling socket rejected")
	}
}
