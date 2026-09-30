package call

import (
	"context"
	"fmt"
	"os"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	"cameo/internal/ent/schema_types/call"
	"cameo/internal/protocol"
	"cameo/internal/tools/ro"
	"cameo/server/services/request_context"
)

// Delete removes a finished call of the couple with its highlights and recordings.
func (h *Handler) Delete(ctx context.Context) error {
	id, err := ro.PathUUID(ctx, "id")
	if err != nil {
		return err
	}

	user, err := currentUser(ctx, h.db)
	if err != nil {
		return err
	}

	row, err := h.db.Call.Query().
		Where(entCall.ID.EQ(id), entCall.CoupleID.EQ(*user.CoupleID)).
		Columns(entCall.ID, entCall.Status).
		Only(ctx)
	if ent.IsNotFound(err) {
		return errNotFound
	}
	if err != nil {
		return fmt.Errorf("load call: %w", err)
	}
	if row.Status == call.StatusRinging || row.Status == call.StatusActive {
		return protocol.ErrorResponse{Code: protocol.CallInvalidState, Message: "call is in progress"}
	}

	err = h.db.Call.DeleteOneID(row.ID).Where(entCall.Status.NotIn(call.StatusRinging, call.StatusActive)).Exec(ctx)
	if ent.IsNotFound(err) {
		return errNotFound
	}
	if err != nil {
		return fmt.Errorf("delete call: %w", err)
	}

	logger := request_context.Logger(ctx)
	if err = h.storage.RemovePrefix(context.WithoutCancel(ctx), fmt.Sprintf("calls/%s/%s/", *user.CoupleID, id)); err != nil {
		logger.Warn().Err(err).Msg("failed to remove call recordings")
	}
	if err = os.RemoveAll(h.call.RecordingDir(id)); err != nil {
		logger.Warn().Err(err).Msg("failed to remove local call recording")
	}

	return nil
}
