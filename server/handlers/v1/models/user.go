package models

import (
	"time"

	"github.com/google/uuid"

	"cameo/internal/ent"
)

type Partner struct {
	ID          uuid.UUID `json:"id"`
	DisplayName *string   `json:"display_name"`
}

type User struct {
	ID             uuid.UUID `json:"id"`
	Phone          string    `json:"phone"`
	DisplayName    *string   `json:"display_name"`
	PairingCode    string    `json:"pairing_code"`
	CallAlert      bool      `json:"call_alert"`
	HighlightAlert bool      `json:"highlight_alert"`
	Partner        *Partner  `json:"partner"`
	CreatedAt      time.Time `json:"created_at"`
}

// FromUser maps u; partner is nil when u is not connected.
func FromUser(u *ent.User, partner *ent.User) User {
	user := User{
		ID:             u.ID,
		Phone:          u.Phone,
		DisplayName:    u.DisplayName,
		PairingCode:    u.PairingCode,
		CallAlert:      u.CallAlert,
		HighlightAlert: u.HighlightAlert,
		CreatedAt:      u.CreatedAt,
	}
	if partner != nil {
		user.Partner = &Partner{ID: partner.ID, DisplayName: partner.DisplayName}
	}
	return user
}
