package config

import validation "github.com/go-ozzo/ozzo-validation/v4"

type RevenueCat struct {
	// APIKey is a public SDK key or a secret key, sent as a bearer token to the REST v1 API.
	APIKey string `json:"api_key"`
	// WebhookSecret signs webhook deliveries. The webhook route is not registered when it is empty.
	WebhookSecret      string `json:"webhook_secret"`
	PremiumEntitlement string `json:"premium_entitlement"`
	RestoreProductID   string `json:"restore_product_id"`
}

func (r *RevenueCat) Validate() error {
	if r.PremiumEntitlement == "" {
		r.PremiumEntitlement = "premium"
	}
	if r.RestoreProductID == "" {
		r.RestoreProductID = "restore"
	}

	return validation.ValidateStruct(r,
		validation.Field(&r.APIKey, validation.Required),
	)
}
