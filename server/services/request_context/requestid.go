//revive:disable:var-naming // package name mirrors directory naming used across server
package request_context

import (
	"context"

	"github.com/google/uuid"
)

func RequestID(ctx context.Context) uuid.UUID {
	id, ok := ctx.Value(keyRequestID).(uuid.UUID)
	if !ok {
		return uuid.Nil
	}

	return id
}

func WithRequestID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}
