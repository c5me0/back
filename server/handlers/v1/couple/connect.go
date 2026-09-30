package couple

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"

	"cameo/internal/ent"
	entUser "cameo/internal/ent/user"
	"cameo/internal/protocol"
	"cameo/server/handlers/v1/models"
	"cameo/server/services/request_context"
)

var errCodeNotFound = protocol.ErrorResponse{Code: protocol.CoupleCodeNotFound, Message: "pairing code not found"}

type ConnectRequest struct {
	Code string `json:"code"`
}

func (r *ConnectRequest) Validate() error {
	return validation.ValidateStruct(r,
		validation.Field(&r.Code, validation.Required, validation.Match(regexp.MustCompile(`^\d{6}$`))),
	)
}

// Connect pairs the authenticated user with the owner of the pairing code.
func (h *Handler) Connect(ctx context.Context, req *ConnectRequest) (*models.Couple, error) {
	myID := request_context.UserID(ctx)

	var me, partner *ent.User
	couple, err := func() (_ *ent.Couple, ret error) {
		tx, commit, err := request_context.Tx(ctx, h.db)
		if err != nil {
			return nil, err
		}
		defer commit(&ret)

		owner, err := tx.User.Query().Where(entUser.PairingCode.EQ(req.Code)).Columns(entUser.ID).Only(ctx)
		if ent.IsNotFound(err) {
			return nil, errCodeNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("load code owner: %w", err)
		}
		if owner.ID == myID {
			return nil, protocol.ErrorResponse{Code: protocol.CoupleSelf, Message: "cannot connect with yourself"}
		}

		// Lock both rows in ascending id order so concurrent connects cannot deadlock.
		users, err := tx.User.Query().
			Where(entUser.ID.In(myID, owner.ID)).
			Order(entUser.ID.Asc()).
			ForUpdate().
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("lock users: %w", err)
		}
		for _, user := range users {
			if user.ID == myID {
				me = user
			} else {
				partner = user
			}
		}
		if me == nil {
			return nil, fmt.Errorf("load user %s: not found", myID)
		}
		if partner == nil || partner.PairingCode != req.Code {
			return nil, errCodeNotFound
		}
		if me.CoupleID != nil {
			return nil, protocol.ErrorResponse{Code: protocol.CoupleAlreadyConnected, Message: "already connected"}
		}
		if partner.CoupleID != nil {
			return nil, protocol.ErrorResponse{Code: protocol.CouplePartnerUnavailable, Message: "partner is already connected"}
		}

		// user_ids is sorted so both orders of the same pair are stored identically.
		userIDs := []uuid.UUID{me.ID, partner.ID}
		slices.SortFunc(userIDs, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })

		// InsertBulk instead of Insert: staticcheck v0.8.0 mis-maps facts of builders with generic methods (SA4023 panic).
		rows, err := tx.Couple.InsertBulk(ent.CoupleInsert{UserIDs: ent.Some(userIDs), MemberIDs: userIDs}).Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("insert couple: %w", err)
		}
		return rows[0], nil
	}()
	if err != nil {
		return nil, err
	}

	// A used code must not connect anyone else, so both codes are replaced once the couple exists.
	for _, userID := range []uuid.UUID{me.ID, partner.ID} {
		if _, err := regeneratePairingCode(ctx, h.db, userID); err != nil {
			return nil, err
		}
	}

	body := "새로운 상대와 연결됐어요"
	if me.DisplayName != nil {
		body = *me.DisplayName + "님과 연결됐어요"
	}
	//nolint:contextcheck // fire-and-forget
	h.push.NotifyUser(partner.ID, false, "연결됐어요", body, map[string]any{"type": "partner_connected"})

	restorable, err := restorable(ctx, h.db, couple)
	if err != nil {
		return nil, err
	}

	result := models.FromCouple(couple, partner, restorable)
	return &result, nil
}
