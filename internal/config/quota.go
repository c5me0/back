package config

import validation "github.com/go-ozzo/ozzo-validation/v4"

// Quota caps the bytes a couple stores. Without it storage is unlimited.
type Quota struct {
	FreeBytes int64 `json:"free_bytes"`
	// Tiers maps a RevenueCat entitlement id to the couple's total quota in bytes while it is active.
	Tiers map[string]int64 `json:"tiers"`
}

func (q *Quota) Validate() error {
	if q.FreeBytes == 0 {
		q.FreeBytes = 1_000_000_000
	}

	return validation.ValidateStruct(q,
		validation.Field(&q.FreeBytes, validation.Min(int64(1))),
		validation.Field(&q.Tiers, validation.Each(validation.Min(q.FreeBytes).Exclusive())),
	)
}
