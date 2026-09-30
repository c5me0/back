package config

import validation "github.com/go-ozzo/ozzo-validation/v4"

type Push struct {
	APNS *APNS `json:"apns"`
}

func (p *Push) Validate() error {
	return validation.ValidateStruct(p,
		validation.Field(&p.APNS),
	)
}

type APNS struct {
	KeyPath    string `json:"key_path"`
	KeyID      string `json:"key_id"`
	TeamID     string `json:"team_id"`
	BundleID   string `json:"bundle_id"`
	Production bool   `json:"production"`
}

func (a *APNS) Validate() error {
	return validation.ValidateStruct(a,
		validation.Field(&a.KeyPath, validation.Required),
		validation.Field(&a.KeyID, validation.Required),
		validation.Field(&a.TeamID, validation.Required),
		validation.Field(&a.BundleID, validation.Required),
	)
}
