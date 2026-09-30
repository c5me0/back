package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"cameo/internal/ent"
	entUser "cameo/internal/ent/user"
	"cameo/internal/protocol"
	"cameo/internal/tools/pairing"
	"cameo/server/handlers/v1/models"
	"cameo/server/services/request_context"
)

const maxInsertAttempts = 5

type PhoneVerifyRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

func (r *PhoneVerifyRequest) Validate() error {
	return validation.ValidateStruct(r,
		validation.Field(&r.Phone, validation.Required, phoneRule),
		validation.Field(&r.Code, validation.Required, validation.Match(regexp.MustCompile(`^\d{4,8}$`))),
	)
}

type PhoneVerifyResponse struct {
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
	IsNew     bool        `json:"is_new"`
	User      models.User `json:"user"`
}

// PhoneVerify checks the code, signs the phone's user in and creates the user on first sign-in.
func (h *Handler) PhoneVerify(ctx context.Context, req *PhoneVerifyRequest) (res *PhoneVerifyResponse, ret error) {
	valid, err := h.otp.Check(ctx, req.Phone, req.Code)
	if err != nil {
		return nil, fmt.Errorf("check verification: %w", err)
	}
	if !valid {
		return nil, protocol.ErrorResponse{Code: protocol.AuthInvalidCode, Message: "invalid or expired code"}
	}

	tx, commit, err := request_context.Tx(ctx, h.db)
	if err != nil {
		return nil, err
	}
	defer commit(&ret)

	// ON CONFLICT DO NOTHING keeps the transaction usable when the pairing code collides or a concurrent
	// sign-in created the same phone first; the next attempt picks up the concurrent user or a fresh code.
	var user *ent.User
	isNew := false
	for range maxInsertAttempts {
		user, err = tx.User.Query().Where(entUser.Phone.EQ(req.Phone)).Only(ctx)
		if !ent.IsNotFound(err) {
			break
		}

		user, err = tx.User.Insert(ent.UserInsert{
			Phone:       req.Phone,
			PairingCode: pairing.Generate(),
		}).OnConflict().DoNothing().Save(ctx)
		if !errors.Is(err, ent.ErrConflict) {
			isNew = true
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("load or create user: %w", err)
	}

	token, expiresAt, err := h.session.Create(ctx, tx, user)
	if err != nil {
		return nil, err
	}

	var partner *ent.User
	if user.CoupleID != nil {
		partner, err = tx.User.Query().
			Where(entUser.CoupleID.EQ(*user.CoupleID), entUser.ID.NEQ(user.ID)).
			Columns(entUser.ID, entUser.DisplayName).
			Only(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return nil, fmt.Errorf("load partner: %w", err)
		}
	}

	return &PhoneVerifyResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		IsNew:     isNew,
		User:      models.FromUser(user, partner),
	}, nil
}
