package models

import (
	"time"

	"github.com/google/uuid"

	"cameo/internal/ent"
)

type CouplePartner struct {
	ID          uuid.UUID `json:"id"`
	DisplayName *string   `json:"display_name"`
	Phone       string    `json:"phone"`
}

// Restorable counts the calls and photos left under the pair's earlier couples.
type Restorable struct {
	Calls  int `json:"calls"`
	Photos int `json:"photos"`
}

type Couple struct {
	ID          uuid.UUID     `json:"id"`
	Partner     CouplePartner `json:"partner"`
	Restorable  Restorable    `json:"restorable"`
	ConnectedAt time.Time     `json:"connected_at"`
	RestoredAt  *time.Time    `json:"restored_at"`
}

func FromCouple(c *ent.Couple, partner *ent.User, restorable Restorable) Couple {
	return Couple{
		ID: c.ID,
		Partner: CouplePartner{
			ID:          partner.ID,
			DisplayName: partner.DisplayName,
			Phone:       partner.Phone,
		},
		Restorable:  restorable,
		ConnectedAt: c.CreatedAt,
		RestoredAt:  c.RestoredAt,
	}
}
