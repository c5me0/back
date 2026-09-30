// Package otp sends and checks phone verification codes.
package otp

import (
	"context"
	"errors"

	"github.com/rs/zerolog"

	"cameo/internal/config"
)

// Provider delivers a one-time code to a phone number (E.164) and checks the code the user entered.
type Provider interface {
	Start(ctx context.Context, phone string) error
	// Check reports whether code is the valid pending code for phone.
	Check(ctx context.Context, phone, code string) (bool, error)
}

// New returns the Twilio provider when configured, otherwise the fake provider.
// The fake provider is refused outside local builds unless otp.allow_fake is set.
func New(cfg *config.Config, logger zerolog.Logger) (Provider, error) {
	if cfg.Twilio != nil {
		return newTwilio(cfg.Twilio), nil
	}

	//goland:noinspection GoBoolExpressions
	if config.Version != "local" && !cfg.OTP.AllowFake {
		return nil, errors.New("twilio is not configured and otp.allow_fake is not set")
	}

	logger.Warn().Str("component", "otp").Msg("twilio is not configured, using the fake OTP provider")
	return newFake(cfg.OTP.FakeCode, logger), nil
}
