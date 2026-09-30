package auth

import (
	"context"
	"fmt"

	entSession "cameo/internal/ent/session"
	"cameo/server/services/request_context"
)

// Signout revokes the current session.
func (h *Handler) Signout(ctx context.Context) error {
	_, err := h.db.Session.Delete().Where(entSession.ID.EQ(request_context.Session(ctx).ID)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}
