package otp

import (
	"context"
	"crypto/subtle"
	"sync"

	"github.com/rs/zerolog"
)

// fakeProvider sends nothing and accepts a single configured code for every phone.
type fakeProvider struct {
	code   string
	logger zerolog.Logger
	warned sync.Once
}

func newFake(code string, logger zerolog.Logger) *fakeProvider {
	return &fakeProvider{code: code, logger: logger}
}

func (p *fakeProvider) Start(_ context.Context, phone string) error {
	p.warned.Do(func() {
		p.logger.Warn().Str("component", "otp").Str("phone", phone).Msg("fake OTP provider: no code is sent, use the configured otp.fake_code")
	})
	return nil
}

func (p *fakeProvider) Check(_ context.Context, _ string, code string) (bool, error) {
	return subtle.ConstantTimeCompare([]byte(code), []byte(p.code)) == 1, nil
}
