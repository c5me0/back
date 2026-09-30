package couple

import (
	"context"

	"cameo/server/services/request_context"
)

type RegenerateCodeResponse struct {
	PairingCode string `json:"pairing_code"`
}

// RegenerateCode replaces the authenticated user's pairing code.
func (h *Handler) RegenerateCode(ctx context.Context) (*RegenerateCodeResponse, error) {
	code, err := regeneratePairingCode(ctx, h.db, request_context.UserID(ctx))
	if err != nil {
		return nil, err
	}
	return &RegenerateCodeResponse{PairingCode: code}, nil
}
