// Package couple handles connecting and disconnecting a couple and the pairing codes used to connect.
package couple

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/neko-sc/ent/dialect"

	"cameo/internal/ent"
	entUser "cameo/internal/ent/user"
	"cameo/internal/protocol"
	"cameo/internal/tools/pairing"
	callService "cameo/server/services/call"
	"cameo/server/services/push"
)

const maxCodeAttempts = 5

var errNotConnected = protocol.ErrorResponse{Code: protocol.CoupleNotConnected, Message: "couple not connected"}

type Handler struct {
	db   *ent.Client
	call *callService.Service
	push *push.Service
}

func NewHandler(db *ent.Client, call *callService.Service, push *push.Service) *Handler {
	return &Handler{db: db, call: call, push: push}
}

// regeneratePairingCode gives the user a fresh pairing code, retrying on collisions.
// It runs outside a transaction because a unique violation aborts the surrounding transaction.
func regeneratePairingCode(ctx context.Context, db *ent.Client, userID uuid.UUID) (string, error) {
	for range maxCodeAttempts {
		code := pairing.Generate()
		err := db.User.UpdateOneID(userID).Set(entUser.PairingCode, code).Exec(ctx)
		if constraintErr, ok := errors.AsType[*dialect.ConstraintError](err); ok && constraintErr.Kind == dialect.Unique && constraintErr.Constraint == "user_pairing_code" {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("update pairing code: %w", err)
		}
		return code, nil
	}
	return "", errors.New("pairing code collided on every attempt")
}
