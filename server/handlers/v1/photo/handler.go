// Package photo handles the couple's shared photo library.
package photo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"cameo/internal/ent"
	entUser "cameo/internal/ent/user"
	"cameo/internal/protocol"
	"cameo/server/services/purchase"
	"cameo/server/services/request_context"
	"cameo/server/services/storage"
)

const (
	maxOriginalSize  = 25 << 20
	maxThumbnailSize = 1 << 20

	uploadExpiry   = 15 * time.Minute
	downloadExpiry = time.Hour
)

var errNotFound = protocol.ErrorResponse{Code: protocol.NotFound, Message: "photo not found"}

type Handler struct {
	db       *ent.Client
	storage  *storage.Service
	purchase *purchase.Service
}

func NewHandler(db *ent.Client, storage *storage.Service, purchase *purchase.Service) *Handler {
	return &Handler{db: db, storage: storage, purchase: purchase}
}

// coupleID returns the authenticated user's couple, or couple:not_connected.
func coupleID(ctx context.Context, client *ent.Client) (uuid.UUID, error) {
	me, err := client.User.Query().
		Where(entUser.ID.EQ(request_context.UserID(ctx))).
		Columns(entUser.ID, entUser.CoupleID).
		Only(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("load user: %w", err)
	}
	if me.CoupleID == nil {
		return uuid.Nil, protocol.ErrorResponse{Code: protocol.CoupleNotConnected, Message: "couple not connected"}
	}

	return *me.CoupleID, nil
}
