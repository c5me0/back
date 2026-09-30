package config

import validation "github.com/go-ozzo/ozzo-validation/v4"

type Service struct {
	CORSAllowedOrigins []string `json:"cors_allowed_origins"`
}

func (s *Service) Validate() error {
	return validation.ValidateStruct(s,
		validation.Field(&s.CORSAllowedOrigins, validation.Each(validation.Length(1, 255))),
	)
}
