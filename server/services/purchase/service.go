// Package purchase mirrors RevenueCat purchases onto users and derives the couple's storage quota and restore credits.
// Purchases are couple-scoped: a purchase by either member counts for both.
package purchase

import (
	"github.com/rs/zerolog"

	"cameo/internal/config"
	"cameo/internal/ent"
)

type Service struct {
	db *ent.Client
	// config is nil when RevenueCat is not configured.
	config *config.RevenueCat
	// quota is nil when storage is unlimited.
	quota  *config.Quota
	logger zerolog.Logger
}

func New(cfg *config.Config, db *ent.Client, logger zerolog.Logger) *Service {
	return &Service{
		db:     db,
		config: cfg.RevenueCat,
		quota:  cfg.Quota,
		logger: logger.With().Str("component", "purchase").Logger(),
	}
}

// Enabled reports whether RevenueCat is configured. When it is not, purchases are never synced and restores are free.
func (s *Service) Enabled() bool {
	return s.config != nil
}
