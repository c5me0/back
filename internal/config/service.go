package config

import validation "github.com/go-ozzo/ozzo-validation/v4"

type Service struct {
	CORSAllowedOrigins []string `json:"cors_allowed_origins"`
	// TrustedProxies is the number of reverse proxies in front of the server whose X-Forwarded-For entries are
	// trusted. Zero uses the connection's remote address as the client IP.
	TrustedProxies int `json:"trusted_proxies"`
}

func (s *Service) Validate() error {
	return validation.ValidateStruct(s,
		validation.Field(&s.CORSAllowedOrigins, validation.Each(validation.Length(1, 255))),
		validation.Field(&s.TrustedProxies, validation.Min(0)),
	)
}
