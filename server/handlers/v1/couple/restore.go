package couple

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/neko-sc/ent/dialect"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	entCouple "cameo/internal/ent/couple"
	entPhoto "cameo/internal/ent/photo"
	entUser "cameo/internal/ent/user"
	"cameo/internal/protocol"
	"cameo/server/handlers/v1/models"
	"cameo/server/services/request_context"
)

var errRestoreRequired = protocol.ErrorResponse{
	Code:    protocol.PurchaseRequired,
	Message: "restore purchase required",
	Meta:    map[string]any{"required": "restore"},
}

// Restore moves the calls and photos of the pair's earlier couples into the current couple, spending one restore purchase.
// A couple restores once; when nothing moves the purchase is not spent.
func (h *Handler) Restore(ctx context.Context) (*models.Restorable, error) {
	me, err := h.db.User.Query().
		Where(entUser.ID.EQ(request_context.UserID(ctx))).
		Columns(entUser.ID, entUser.CoupleID, entUser.DisplayName).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}
	if me.CoupleID == nil {
		return nil, errNotConnected
	}

	// Syncing first picks up a purchase the webhook has not delivered yet. Disabled purchases restore for free.
	var transactionID *string
	if h.purchase.Enabled() {
		if _, err = h.purchase.Sync(ctx, me.ID); err != nil {
			return nil, err
		}
		status, err := h.purchase.Status(ctx, me)
		if err != nil {
			return nil, err
		}
		if status.RestoreCredits() > 0 {
			transactionID = &status.RestoreTransactionIDs[0]
		}
	}

	result, err := func() (_ *models.Restorable, ret error) {
		tx, commit, err := request_context.Tx(ctx, h.db)
		if err != nil {
			return nil, err
		}
		defer commit(&ret)

		couple, err := tx.Couple.Query().Where(entCouple.ID.EQ(*me.CoupleID)).ForUpdate().Only(ctx)
		if err != nil {
			return nil, fmt.Errorf("lock couple: %w", err)
		}
		if couple.RestoredAt != nil {
			return &models.Restorable{}, nil
		}

		previous, err := previousCoupleIDs(ctx, tx.Client(), couple)
		if err != nil || len(previous) == 0 {
			return &models.Restorable{}, err
		}

		// Count and Exec instead of Save: staticcheck v0.8.0 mis-maps facts of builders with generic methods (SA4023 panic).
		calls, err := tx.Call.Query().Where(entCall.CoupleID.In(previous...)).Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("count calls: %w", err)
		}
		photos, err := tx.Photo.Query().Where(entPhoto.CoupleID.In(previous...)).Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("count photos: %w", err)
		}
		if calls == 0 && photos == 0 {
			return &models.Restorable{}, nil
		}
		// Checked after restored_at so an already restored couple answers zeros without a credit.
		if h.purchase.Enabled() && transactionID == nil {
			return nil, errRestoreRequired
		}

		if err = tx.Call.Update().Where(entCall.CoupleID.In(previous...)).Set(entCall.CoupleID, couple.ID).Exec(ctx); err != nil {
			return nil, fmt.Errorf("move calls: %w", err)
		}
		if err = tx.Photo.Update().Where(entPhoto.CoupleID.In(previous...)).Set(entPhoto.CoupleID, couple.ID).Exec(ctx); err != nil {
			return nil, fmt.Errorf("move photos: %w", err)
		}

		// The unique index on restore_transaction_id rejects a purchase another request spent concurrently.
		err = tx.Couple.UpdateOneID(couple.ID).Apply(ent.CouplePatch{
			RestoredAt:           ent.Some(time.Now()),
			RestoreTransactionID: ent.FromPtr(transactionID),
		}).Exec(ctx)
		if constraintErr, ok := errors.AsType[*dialect.ConstraintError](err); ok && constraintErr.Kind == dialect.Unique && constraintErr.Constraint == "idx_couple_restore_transaction" {
			return nil, errRestoreRequired
		}
		if err != nil {
			return nil, fmt.Errorf("update couple: %w", err)
		}
		return &models.Restorable{Calls: calls, Photos: photos}, nil
	}()
	if err != nil {
		return nil, err
	}
	if result.Calls == 0 && result.Photos == 0 {
		return result, nil
	}

	partner, err := h.db.User.Query().
		Where(entUser.CoupleID.EQ(*me.CoupleID), entUser.ID.NEQ(me.ID)).
		Columns(entUser.ID).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("load partner: %w", err)
	}
	if partner != nil {
		title := ""
		if me.DisplayName != nil {
			title = *me.DisplayName
		}
		//nolint:contextcheck // push delivery runs in the background, detached from ctx
		h.push.NotifyUser(partner.ID, false, title, "지난 추억을 복원했어요", map[string]any{
			"type":      "data_restored",
			"couple_id": *me.CoupleID,
			"calls":     result.Calls,
			"photos":    result.Photos,
		})
	}

	return result, nil
}
