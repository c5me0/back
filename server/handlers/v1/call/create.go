package call

import (
	"context"
	"errors"
	"fmt"

	"github.com/neko-sc/ent/dialect"

	"cameo/internal/config"
	"cameo/internal/ent"
	entUser "cameo/internal/ent/user"
	"cameo/internal/protocol"
	"cameo/server/handlers/v1/models"
)

type CreateResponse struct {
	Call       models.Call        `json:"call"`
	ICEServers []config.ICEServer `json:"ice_servers"`
}

// Create starts ringing the partner.
func (h *Handler) Create(ctx context.Context) (*CreateResponse, error) {
	user, err := currentUser(ctx, h.db)
	if err != nil {
		return nil, err
	}

	partner, err := h.db.User.Query().
		Where(entUser.CoupleID.EQ(*user.CoupleID), entUser.ID.NEQ(user.ID)).
		Columns(entUser.ID).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, protocol.ErrorResponse{Code: protocol.CoupleNotConnected, Message: "couple not connected"}
	}
	if err != nil {
		return nil, fmt.Errorf("load partner: %w", err)
	}

	// InsertBulk instead of Insert: staticcheck v0.8.0 mis-maps facts of builders with generic methods (SA4023 panic).
	rows, err := h.db.Call.InsertBulk(ent.CallInsert{
		CoupleID: *user.CoupleID,
		CallerID: user.ID,
		CalleeID: partner.ID,
	}).Save(ctx)
	if constraintErr, ok := errors.AsType[*dialect.ConstraintError](err); ok && constraintErr.Kind == dialect.Unique && constraintErr.Constraint == "idx_call_live" {
		return nil, protocol.ErrorResponse{Code: protocol.CallBusy, Message: "a call is already in progress"}
	}
	if err != nil {
		return nil, fmt.Errorf("insert call: %w", err)
	}

	row := rows[0]

	callerName := ""
	if user.DisplayName != nil {
		callerName = *user.DisplayName
	}
	h.call.Open(ctx, row, callerName)

	return &CreateResponse{
		Call:       models.FromCall(row, user.ID, 0),
		ICEServers: h.iceServers,
	}, nil
}
