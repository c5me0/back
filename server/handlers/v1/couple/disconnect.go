package couple

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	entCouple "cameo/internal/ent/couple"
	"cameo/internal/ent/schema_types/call"
	entUser "cameo/internal/ent/user"
	"cameo/server/services/request_context"
)

// Disconnect dissolves the authenticated user's couple and ends its live call.
func (h *Handler) Disconnect(ctx context.Context) error {
	myID := request_context.UserID(ctx)

	var coupleID, partnerID uuid.UUID
	err := func() (ret error) {
		tx, commit, err := request_context.Tx(ctx, h.db)
		if err != nil {
			return err
		}
		defer commit(&ret)

		me, err := tx.User.Query().Where(entUser.ID.EQ(myID)).Columns(entUser.ID, entUser.CoupleID).Only(ctx)
		if err != nil {
			return fmt.Errorf("load user: %w", err)
		}
		if me.CoupleID == nil {
			return errNotConnected
		}
		coupleID = *me.CoupleID

		// Lock both members in ascending id order, matching Connect, then confirm the couple still holds me.
		members, err := tx.User.Query().
			Where(entUser.CoupleID.EQ(coupleID)).
			Order(entUser.ID.Asc()).
			Columns(entUser.ID, entUser.CoupleID).
			ForUpdate().
			All(ctx)
		if err != nil {
			return fmt.Errorf("lock members: %w", err)
		}
		found := false
		for _, member := range members {
			if member.ID == myID {
				found = true
			} else {
				partnerID = member.ID
			}
		}
		if !found {
			return errNotConnected
		}

		if err := tx.User.Update().Where(entUser.CoupleID.EQ(coupleID)).Clear(entUser.CoupleID).Exec(ctx); err != nil {
			return fmt.Errorf("clear couple: %w", err)
		}
		if err := tx.Couple.UpdateOneID(coupleID).Set(entCouple.DisconnectedAt, time.Now()).Exec(ctx); err != nil {
			return fmt.Errorf("update couple: %w", err)
		}
		return nil
	}()
	if err != nil {
		return err
	}

	// After commit no new call can start for the couple; ForceEnd writes through its own client.
	live, err := h.db.Call.Query().
		Where(entCall.CoupleID.EQ(coupleID), entCall.Status.In(call.StatusRinging, call.StatusActive)).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("load live call: %w", err)
	}
	if live != nil {
		if err := h.call.ForceEnd(ctx, live); err != nil {
			return fmt.Errorf("end live call: %w", err)
		}
	}

	if partnerID != uuid.Nil {
		//nolint:contextcheck // fire-and-forget
		h.push.NotifyUser(partnerID, false, "연결이 해제됐어요", "", map[string]any{"type": "partner_disconnected"})
	}
	return nil
}
