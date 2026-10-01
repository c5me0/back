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

// Storage is the couple's storage usage and quota. QuotaBytes is null when unlimited; Source is "self", "partner" or "none".
type Storage struct {
	UsedBytes  int64      `json:"used_bytes"`
	QuotaBytes *int64     `json:"quota_bytes"`
	Tier       *string    `json:"tier"`
	Source     string     `json:"source"`
	Until      *time.Time `json:"until"`
}

type User struct {
	ID             uuid.UUID `json:"id"`
	Phone          string    `json:"phone"`
	DisplayName    *string   `json:"display_name"`
	PairingCode    string    `json:"pairing_code"`
	CallAlert      bool      `json:"call_alert"`
	HighlightAlert bool      `json:"highlight_alert"`
	Partner        *Partner  `json:"partner"`
	Storage        Storage   `json:"storage"`
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
		Storage: Storage{
			UsedBytes:  status.Storage.UsedBytes,
			QuotaBytes: status.Storage.QuotaBytes,
			Tier:       status.Storage.Tier,
			Source:     status.Storage.Source,
			Until:      status.Storage.Until,
		},
		RestoreCredits: status.RestoreCredits(),
		CreatedAt:      u.CreatedAt,
	}
	if partner != nil {
		user.Partner = &Partner{ID: partner.ID, DisplayName: partner.DisplayName}
	}
	return user
}
