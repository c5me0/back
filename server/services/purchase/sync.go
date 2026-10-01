package purchase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"cameo/internal/ent"
)

// lifetime stands in for the expiry of an entitlement that never expires.
var lifetime = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)

// Sync copies the user's largest active storage tier and restore purchases from RevenueCat onto the user row and returns the reloaded user.
func (s *Service) Sync(ctx context.Context, userID uuid.UUID) (*ent.User, error) {
	if !s.Enabled() {
		return s.loadUser(ctx, userID)
	}

	subscriber, err := s.fetchSubscriber(ctx, userID)
	if err != nil {
		return nil, err
	}

	var storageEntitlement *string
	var storageUntil *time.Time
	if s.quota != nil {
		now := time.Now()
		for entitlementID, entitlement := range subscriber.Entitlements {
			quotaBytes, ok := s.quota.Tiers[entitlementID]
			until := entitlement.until()
			if !ok || !until.After(now) {
				continue
			}
			if storageEntitlement == nil || quotaBytes > s.quota.Tiers[*storageEntitlement] {
				storageEntitlement, storageUntil = &entitlementID, &until
			}
		}
	}

	restorePurchases := subscriber.NonSubscriptions[s.config.RestoreProductID]
	restoreTransactionIDs := make([]string, 0, len(restorePurchases))
	for _, restorePurchase := range restorePurchases {
		restoreTransactionIDs = append(restoreTransactionIDs, restorePurchase.ID)
	}

	// Exec and reload instead of Save: staticcheck v0.8.0 SA4023 panics on UserUpdateOne.Save.
	err = s.db.User.UpdateOneID(userID).Apply(ent.UserPatch{
		StorageEntitlement:    ent.NullIfNil(storageEntitlement),
		StorageUntil:          ent.NullIfNil(storageUntil),
		RestoreTransactionIDs: ent.Some(restoreTransactionIDs),
		PurchasesSyncedAt:     ent.Some(time.Now()),
	}).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update user purchases: %w", err)
	}

	return s.loadUser(ctx, userID)
}

func (s *Service) loadUser(ctx context.Context, userID uuid.UUID) (*ent.User, error) {
	user, err := s.db.User.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user: %w", err)
	}
	return user, nil
}
