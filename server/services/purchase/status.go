package purchase

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	entCouple "cameo/internal/ent/couple"
	entPhoto "cameo/internal/ent/photo"
	"cameo/internal/ent/schema_types/photo"
	entUser "cameo/internal/ent/user"
	"cameo/internal/protocol"
)

// Storage tier sources reported by Status.
const (
	SourceSelf    = "self"
	SourcePartner = "partner"
	SourceNone    = "none"
)

var (
	photoBytes     = ent.Sum(entPhoto.SizeBytes).Nullable()
	thumbnailBytes = ent.Sum(entPhoto.ThumbnailSizeBytes).Nullable()
	recordingBytes = ent.Sum(entCall.RecordingBytes).Nullable()
)

type Storage struct {
	// UsedBytes counts uploaded photos with their thumbnails and call recordings of the couple.
	UsedBytes int64
	// QuotaBytes is nil when storage is unlimited.
	QuotaBytes *int64
	// Tier is the active entitlement raising the quota, nil on the free tier.
	Tier *string
	// Source is who pays for the tier: SourceSelf, SourcePartner or SourceNone.
	Source string
	// Until is the tier's expiry, nil on the free tier.
	Until *time.Time
}

// Reserve admits storing required more bytes, or returns storage:quota_exceeded.
func (s Storage) Reserve(required int64) error {
	if s.QuotaBytes == nil || s.UsedBytes+required <= *s.QuotaBytes {
		return nil
	}
	return protocol.ErrorResponse{
		Code:    protocol.StorageQuotaExceeded,
		Message: "storage quota exceeded",
		Meta:    map[string]any{"used_bytes": s.UsedBytes, "quota_bytes": *s.QuotaBytes, "required_bytes": required},
	}
}

type Status struct {
	Storage Storage
	// RestoreTransactionIDs are the members' restore purchases not yet consumed by any couple, sorted ascending.
	RestoreTransactionIDs []string
}

// RestoreCredits is the number of restore purchases the couple can still spend.
func (s Status) RestoreCredits() int {
	return len(s.RestoreTransactionIDs)
}

// Status combines the purchases and storage of me's couple, or of me alone when not connected. It reads only me.ID and me.CoupleID.
func (s *Service) Status(ctx context.Context, me *ent.User) (Status, error) {
	membership := entUser.ID.EQ(me.ID)
	if me.CoupleID != nil {
		membership = entUser.CoupleID.EQ(*me.CoupleID)
	}
	members, err := s.db.User.Query().
		Where(membership).
		Columns(entUser.ID, entUser.StorageEntitlement, entUser.StorageUntil, entUser.RestoreTransactionIDs).
		All(ctx)
	if err != nil {
		return Status{}, fmt.Errorf("load members: %w", err)
	}

	status := Status{Storage: Storage{Source: SourceNone}}
	if s.quota != nil {
		status.Storage.QuotaBytes = new(s.quota.FreeBytes)
	}
	now := time.Now()
	var purchased []string
	for _, member := range members {
		purchased = append(purchased, member.RestoreTransactionIDs...)

		if s.quota == nil || member.StorageEntitlement == nil || member.StorageUntil == nil || !member.StorageUntil.After(now) {
			continue
		}
		quotaBytes, ok := s.quota.Tiers[*member.StorageEntitlement]
		if !ok || quotaBytes <= *status.Storage.QuotaBytes {
			continue
		}
		status.Storage.QuotaBytes = &quotaBytes
		status.Storage.Tier = member.StorageEntitlement
		status.Storage.Until = member.StorageUntil
		status.Storage.Source = SourcePartner
		if member.ID == me.ID {
			status.Storage.Source = SourceSelf
		}
	}

	if me.CoupleID != nil {
		if status.Storage.UsedBytes, err = s.usedBytes(ctx, *me.CoupleID); err != nil {
			return Status{}, err
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

// usedBytes sums the couple's uploaded photos with their thumbnails and its call recordings.
func (s *Service) usedBytes(ctx context.Context, coupleID uuid.UUID) (int64, error) {
	photos, err := s.db.Photo.Query().
		Where(entPhoto.CoupleID.EQ(coupleID), entPhoto.Status.EQ(photo.StatusUploaded)).
		Aggregate(photoBytes, thumbnailBytes).
		Row(ctx)
	if err != nil {
		return 0, fmt.Errorf("sum photo bytes: %w", err)
	}
	calls, err := s.db.Call.Query().
		Where(entCall.CoupleID.EQ(coupleID)).
		Aggregate(recordingBytes).
		Row(ctx)
	if err != nil {
		return 0, fmt.Errorf("sum recording bytes: %w", err)
	}

	// SUM over no rows is NULL, which reads as zero.
	original, _ := ent.GetNullable(photos, photoBytes)
	thumbnail, _ := ent.GetNullable(photos, thumbnailBytes)
	recording, _ := ent.GetNullable(calls, recordingBytes)
	return original + thumbnail + recording, nil
}
