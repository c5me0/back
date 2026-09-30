//revive:disable:var-naming // package name mirrors directory naming used across server
package request_context

import (
	"context"

	"github.com/google/uuid"
)

// UserID returns the authenticated user's ID, or uuid.Nil outside the session middleware.
func UserID(ctx context.Context) uuid.UUID {
	userID, ok := ctx.Value(keyUserID).(uuid.UUID)
	if !ok {
		return uuid.Nil
	}

	return userID
}

func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, keyUserID, userID)
}
