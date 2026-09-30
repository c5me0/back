package otp

import (
	"context"
	"errors"
	"fmt"

	"github.com/twilio/twilio-go"
	twilioClient "github.com/twilio/twilio-go/client"
	verify "github.com/twilio/twilio-go/rest/verify/v2"

	"cameo/internal/config"
)

// twilioNotFound is returned by Verify when no pending verification exists (expired, approved, or too many attempts).
const twilioNotFound = 20404

type twilioProvider struct {
	client     *twilio.RestClient
	serviceSID string
}

func newTwilio(cfg *config.Twilio) *twilioProvider {
	return &twilioProvider{
		client: twilio.NewRestClientWithParams(twilio.ClientParams{
			Username: cfg.AccountSID,
			Password: cfg.AuthToken,
		}),
		serviceSID: cfg.VerifyServiceSID,
	}
}

func (p *twilioProvider) Start(_ context.Context, phone string) error {
	params := &verify.CreateVerificationParams{}
	if _, err := p.client.VerifyV2.CreateVerification(p.serviceSID, params.SetTo(phone).SetChannel("sms")); err != nil {
		return fmt.Errorf("twilio create verification: %w", err)
	}
	return nil
}

func (p *twilioProvider) Check(_ context.Context, phone, code string) (bool, error) {
	params := &verify.CreateVerificationCheckParams{}
	check, err := p.client.VerifyV2.CreateVerificationCheck(p.serviceSID, params.SetTo(phone).SetCode(code))
	if err != nil {
		if restErr, ok := errors.AsType[*twilioClient.TwilioRestError](err); ok && restErr.Code == twilioNotFound {
			return false, nil
		}
		return false, fmt.Errorf("twilio create verification check: %w", err)
	}
	return check.Status != nil && *check.Status == "approved", nil
}
