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

type Couple struct {
	ID          uuid.UUID     `json:"id"`
	Partner     CouplePartner `json:"partner"`
	ConnectedAt time.Time     `json:"connected_at"`
}

func FromCouple(c *ent.Couple, partner *ent.User) Couple {
	return Couple{
		ID: c.ID,
		Partner: CouplePartner{
			ID:          partner.ID,
			DisplayName: partner.DisplayName,
			Phone:       partner.Phone,
		},
		ConnectedAt: c.CreatedAt,
	}
}
