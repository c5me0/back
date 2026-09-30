package models

import (
	"time"

	"github.com/google/uuid"

	"cameo/internal/ent"
	"cameo/server/services/purchase"
)

type Partner struct {
	ID          uuid.UUID `json:"id"`
	DisplayName *string   `json:"display_name"`
}

// Premium is the couple's premium entitlement. Source is "self", "partner" or "none".
type Premium struct {
	Active bool       `json:"active"`
	Until  *time.Time `json:"until"`
	Source string     `json:"source"`
}

type User struct {
	ID             uuid.UUID `json:"id"`
	Phone          string    `json:"phone"`
	DisplayName    *string   `json:"display_name"`
	PairingCode    string    `json:"pairing_code"`
	CallAlert      bool      `json:"call_alert"`
	HighlightAlert bool      `json:"highlight_alert"`
	Partner        *Partner  `json:"partner"`
	Premium        Premium   `json:"premium"`
	RestoreCredits int       `json:"restore_credits"`
	CreatedAt      time.Time `json:"created_at"`
}

// FromUser maps u with its couple's purchase status; partner is nil when u is not connected.
func FromUser(u *ent.User, partner *ent.User, status purchase.Status) User {
	user := User{
		ID:             u.ID,
		Phone:          u.Phone,
		DisplayName:    u.DisplayName,
		PairingCode:    u.PairingCode,
		CallAlert:      u.CallAlert,
		HighlightAlert: u.HighlightAlert,
		Premium: Premium{
			Active: status.PremiumActive,
			Until:  status.PremiumUntil,
			Source: status.PremiumSource,
		},
		RestoreCredits: status.RestoreCredits(),
		CreatedAt:      u.CreatedAt,
	}
	if partner != nil {
		user.Partner = &Partner{ID: partner.ID, DisplayName: partner.DisplayName}
	}
	return user
}
