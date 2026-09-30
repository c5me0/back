package push

import (
	"context"

	"github.com/rs/zerolog"
)

// logProvider only logs notifications. It is used when APNs is not configured.
type logProvider struct {
	logger zerolog.Logger
}

func (p *logProvider) Send(_ context.Context, n Notification) ([]string, error) {
	p.logger.Info().
		Int("tokens", len(n.Tokens)).
		Bool("voip", n.VoIP).
		Str("title", n.Title).
		Str("body", n.Body).
		Interface("data", n.Data).
		Msg("push notification (log provider)")
	return nil, nil
}
