package purchase

import (
	"context"
	"fmt"
	"slices"
	"time"

	"cameo/internal/ent"
	entCouple "cameo/internal/ent/couple"
	entUser "cameo/internal/ent/user"
)

// Premium sources reported by Status.
const (
	SourceSelf    = "self"
	SourcePartner = "partner"
	SourceNone    = "none"
)

type Status struct {
	PremiumActive bool
	// PremiumUntil is the latest active expiry among the members, nil when premium is not active.
	PremiumUntil *time.Time
	// PremiumSource is who pays for the active premium: SourceSelf, SourcePartner or SourceNone.
	PremiumSource string
	// RestoreTransactionIDs are the members' restore purchases not yet consumed by any couple, sorted ascending.
	RestoreTransactionIDs []string
}

// RestoreCredits is the number of restore purchases the couple can still spend.
func (s Status) RestoreCredits() int {
	return len(s.RestoreTransactionIDs)
}

// Status combines the purchases of me's couple members, or of me alone when not connected. It reads only me.ID and me.CoupleID.
func (s *Service) Status(ctx context.Context, me *ent.User) (Status, error) {
	if !s.Enabled() {
		return Status{PremiumActive: true, PremiumSource: SourceNone}, nil
	}

	membership := entUser.ID.EQ(me.ID)
	if me.CoupleID != nil {
		membership = entUser.CoupleID.EQ(*me.CoupleID)
	}
	members, err := s.db.User.Query().
		Where(membership).
		Columns(entUser.ID, entUser.PremiumUntil, entUser.RestoreTransactionIDs).
		All(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("load members: %w", err)
	}

	now := time.Now()
	status := Status{PremiumSource: SourceNone}
	var purchased []string
	for _, member := range members {
		purchased = append(purchased, member.RestoreTransactionIDs...)

		if member.PremiumUntil == nil || !member.PremiumUntil.After(now) {
			continue
		}
		status.PremiumActive = true
		if status.PremiumUntil == nil || member.PremiumUntil.After(*status.PremiumUntil) {
			status.PremiumUntil = member.PremiumUntil
		}
		if member.ID == me.ID {
			status.PremiumSource = SourceSelf
		} else if status.PremiumSource == SourceNone {
			status.PremiumSource = SourcePartner
		}
	}

	if len(purchased) == 0 {
		return status, nil
	}
	slices.Sort(purchased)
	purchased = slices.Compact(purchased)

	consumed, err := s.db.Couple.Query().
		Where(entCouple.RestoreTransactionID.In(purchased...)).
		Columns(entCouple.RestoreTransactionID).
		All(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("load consumed restores: %w", err)
	}
	status.RestoreTransactionIDs = slices.DeleteFunc(purchased, func(transactionID string) bool {
		return slices.ContainsFunc(consumed, func(couple *ent.Couple) bool { return *couple.RestoreTransactionID == transactionID })
	})
	return status, nil
}
