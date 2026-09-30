package config

import validation "github.com/go-ozzo/ozzo-validation/v4"

type Twilio struct {
	AccountSID       string `json:"account_sid"`
	AuthToken        string `json:"auth_token"`
	VerifyServiceSID string `json:"verify_service_sid"`
}

func (t *Twilio) Validate() error {
	return validation.ValidateStruct(t,
		validation.Field(&t.AccountSID, validation.Required),
		validation.Field(&t.AuthToken, validation.Required),
		validation.Field(&t.VerifyServiceSID, validation.Required),
	)
}

type OTP struct {
	// FakeCode is the code accepted by the fake provider used when Twilio is not configured.
	FakeCode string `json:"fake_code"`
	// AllowFake permits the fake provider in non-local builds.
	AllowFake bool `json:"allow_fake"`
}

func (o *OTP) Validate() error {
	if o.FakeCode == "" {
		o.FakeCode = "000000"
	}
	return nil
}
