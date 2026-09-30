package purchase

import (
	"fmt"
	"net/http"

	entUser "cameo/internal/ent/user"
	"cameo/internal/protocol"
	"cameo/internal/tools/ro"
	"cameo/server/services/request_context"
)

var (
	errNotConnected    = protocol.ErrorResponse{Code: protocol.CoupleNotConnected, Message: "couple not connected"}
	errPremiumRequired = protocol.ErrorResponse{
		Code:    protocol.PurchaseRequired,
		Message: "premium required",
		Meta:    map[string]any{"required": "premium"},
	}
)

// Middleware admits only connected users whose couple has an active premium entitlement. It runs after the session middleware.
func (s *Service) Middleware(next http.Handler) http.Handler {
	if !s.Enabled() {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		me, err := s.db.User.Query().
			Where(entUser.ID.EQ(request_context.UserID(ctx))).
			Columns(entUser.ID, entUser.CoupleID).
			Only(ctx)
		if err != nil {
			ro.WriteError(w, r, fmt.Errorf("load user: %w", err))
			return
		}
		if me.CoupleID == nil {
			ro.WriteError(w, r, errNotConnected)
			return
		}

		status, err := s.Status(ctx, me)
		if err != nil {
			ro.WriteError(w, r, err)
			return
		}
		if !status.PremiumActive {
			ro.WriteError(w, r, errPremiumRequired)
			return
		}

		next.ServeHTTP(w, r)
	})
}
