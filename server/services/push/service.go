package push

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"cameo/internal/config"
	"cameo/internal/ent"
	entDevice "cameo/internal/ent/device"
	"cameo/internal/ent/schema_types/device"
)

const sendTimeout = 10 * time.Second

type Service struct {
	db       *ent.Client
	provider Provider
	logger   zerolog.Logger
	pending  sync.WaitGroup
}

// New uses APNs when push.apns is configured, otherwise a provider that only logs.
func New(cfg *config.Config, db *ent.Client, logger zerolog.Logger) (*Service, error) {
	logger = logger.With().Str("component", "push").Logger()

	var provider Provider = &logProvider{logger: logger}
	if cfg.Push != nil && cfg.Push.APNS != nil {
		apns, err := newAPNS(cfg.Push.APNS)
		if err != nil {
			return nil, err
		}
		provider = apns
	}

	return &Service{db: db, provider: provider, logger: logger}, nil
}

// NotifyUser sends a notification to the user's APNs devices (PushKit VoIP devices when voip is set)
// in the background, and deletes devices whose tokens APNs reports as invalid.
func (s *Service) NotifyUser(userID uuid.UUID, voip bool, title, body string, data map[string]any) {
	s.pending.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		defer cancel()

		if err := s.notifyUser(ctx, userID, voip, title, body, data); err != nil {
			s.logger.Warn().Err(err).Str("user_id", userID.String()).Bool("voip", voip).Msg("failed to send push notification")
		}
	})
}

func (s *Service) notifyUser(ctx context.Context, userID uuid.UUID, voip bool, title, body string, data map[string]any) error {
	platform := device.PlatformAPNS
	if voip {
		platform = device.PlatformAPNSVoIP
	}

	devices, err := s.db.Device.Query().
		Where(entDevice.UserID.EQ(userID), entDevice.Platform.EQ(platform)).
		Columns(entDevice.Token).
		All(ctx)
	if err != nil {
		return fmt.Errorf("query devices: %w", err)
	}
	tokens := make([]string, 0, len(devices))
	for _, row := range devices {
		tokens = append(tokens, row.Token)
	}
	if len(tokens) == 0 {
		return nil
	}

	invalid, sendErr := s.provider.Send(ctx, Notification{Tokens: tokens, VoIP: voip, Title: title, Body: body, Data: data})
	if len(invalid) > 0 {
		if _, err = s.db.Device.Delete().
			Where(entDevice.Platform.EQ(platform), entDevice.Token.In(invalid...)).
			Exec(ctx); err != nil {
			return fmt.Errorf("delete invalid devices: %w", err)
		}
	}
	return sendErr
}

// Shutdown waits for in-flight notifications.
func (s *Service) Shutdown() {
	s.pending.Wait()
}
