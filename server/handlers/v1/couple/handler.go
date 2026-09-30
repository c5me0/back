// Package couple handles connecting and disconnecting a couple and the pairing codes used to connect.
package couple

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/neko-sc/ent/dialect"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	entCouple "cameo/internal/ent/couple"
	entPhoto "cameo/internal/ent/photo"
	entUser "cameo/internal/ent/user"
	"cameo/internal/protocol"
	"cameo/internal/tools/pairing"
	"cameo/server/handlers/v1/models"
	callService "cameo/server/services/call"
	"cameo/server/services/purchase"
	"cameo/server/services/push"
)

const maxCodeAttempts = 5

var errNotConnected = protocol.ErrorResponse{Code: protocol.CoupleNotConnected, Message: "couple not connected"}

type Handler struct {
	db       *ent.Client
	call     *callService.Service
	push     *push.Service
	purchase *purchase.Service
}

func NewHandler(db *ent.Client, call *callService.Service, push *push.Service, purchase *purchase.Service) *Handler {
	return &Handler{db: db, call: call, push: push, purchase: purchase}
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

// previousCoupleIDs returns the earlier couples formed by the same two users as current.
// Couples created before user_ids existed have none recorded and match nothing.
func previousCoupleIDs(ctx context.Context, db *ent.Client, current *ent.Couple) ([]uuid.UUID, error) {
	if len(current.UserIDs) == 0 {
		return nil, nil
	}

	ids, err := db.Couple.Query().
		Where(entCouple.UserIDs.Contains(current.UserIDs...), entCouple.ID.NEQ(current.ID)).
		IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("load previous couples: %w", err)
	}
	return ids, nil
}

// restorable counts the calls and photos Restore would move into current.
func restorable(ctx context.Context, db *ent.Client, current *ent.Couple) (models.Restorable, error) {
	previous, err := previousCoupleIDs(ctx, db, current)
	if err != nil || len(previous) == 0 {
		return models.Restorable{}, err
	}

	calls, err := db.Call.Query().Where(entCall.CoupleID.In(previous...)).Count(ctx)
	if err != nil {
		return models.Restorable{}, fmt.Errorf("count restorable calls: %w", err)
	}
	photos, err := db.Photo.Query().Where(entPhoto.CoupleID.In(previous...)).Count(ctx)
	if err != nil {
		return models.Restorable{}, fmt.Errorf("count restorable photos: %w", err)
	}
	return models.Restorable{Calls: calls, Photos: photos}, nil
}
