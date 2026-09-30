package auth

import (
	"context"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type PhoneStartRequest struct {
	Phone string `json:"phone"`
}

func (r *PhoneStartRequest) Validate() error {
	return validation.ValidateStruct(r,
		validation.Field(&r.Phone, validation.Required, phoneRule),
	)
}

// PhoneStart sends a verification code to the phone.
func (h *Handler) PhoneStart(ctx context.Context, req *PhoneStartRequest) error {
	if err := h.otp.Start(ctx, req.Phone); err != nil {
		return fmt.Errorf("start verification: %w", err)
	}

	return nil
}
