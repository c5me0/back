// Package call handles call history, live call actions and the signaling socket.
package call

import (
	"context"
	"fmt"
	"time"

	"cameo/internal/config"
	"cameo/internal/ent"
	entUser "cameo/internal/ent/user"
	"cameo/internal/protocol"
	callService "cameo/server/services/call"
	"cameo/server/services/push"
	"cameo/server/services/request_context"
	"cameo/server/services/storage"
)

const downloadExpiry = time.Hour

var (
	errNotFound  = protocol.ErrorResponse{Code: protocol.NotFound, Message: "call not found"}
	errNotActive = protocol.ErrorResponse{Code: protocol.CallInvalidState, Message: "call is not active"}
)

type Handler struct {
	db         *ent.Client
	storage    *storage.Service
	push       *push.Service
	call       *callService.Service
	iceServers []config.ICEServer
}

func NewHandler(db *ent.Client, storage *storage.Service, push *push.Service, call *callService.Service, iceServers []config.ICEServer) *Handler {
	return &Handler{db: db, storage: storage, push: push, call: call, iceServers: iceServers}
}

// currentUser loads the authenticated user, or returns couple:not_connected when they have no partner.
// A non-nil user always has CoupleID set.
func currentUser(ctx context.Context, client *ent.Client) (*ent.User, error) {
	user, err := client.User.Query().
		Where(entUser.ID.EQ(request_context.UserID(ctx))).
		Columns(entUser.ID, entUser.CoupleID, entUser.DisplayName).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}
	if user.CoupleID == nil {
		return nil, protocol.ErrorResponse{Code: protocol.CoupleNotConnected, Message: "couple not connected"}
	}

	return user, nil
}
