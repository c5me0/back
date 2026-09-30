package account

import (
	"context"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"cameo/internal/ent"
	"cameo/server/handlers/v1/models"
	"cameo/server/services/request_context"
)

type UpdateMeRequest struct {
	DisplayName    *string `json:"display_name"`
	CallAlert      *bool   `json:"call_alert"`
	HighlightAlert *bool   `json:"highlight_alert"`
}

// Validate trims display_name before checking its length.
func (r *UpdateMeRequest) Validate() error {
	if r.DisplayName != nil {
		*r.DisplayName = strings.TrimSpace(*r.DisplayName)
	}

	return validation.ValidateStruct(r,
		validation.Field(&r.DisplayName, validation.When(r.DisplayName != nil, validation.Required), validation.RuneLength(1, 30)),
	)
}

// UpdateMe changes the authenticated user's profile and alert preferences.
func (h *Handler) UpdateMe(ctx context.Context, req *UpdateMeRequest) (*models.User, error) {
	// Exec and reload instead of Save: staticcheck v0.8.0 SA4023 panics on UserUpdateOne.Save.
	err := h.db.User.UpdateOneID(request_context.UserID(ctx)).Apply(ent.UserPatch{
		DisplayName:    ent.FromPtr(req.DisplayName),
		CallAlert:      ent.FromPtr(req.CallAlert),
		HighlightAlert: ent.FromPtr(req.HighlightAlert),
	}).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return h.Me(ctx)
}
