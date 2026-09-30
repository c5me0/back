//revive:disable:var-naming // package name mirrors directory naming used across server
package request_context

import (
	"context"

	"cameo/internal/ent"
)

// Session returns the authenticated session. It panics outside the session middleware.
func Session(ctx context.Context) *ent.Session {
	value, ok := ctx.Value(keySession).(*ent.Session)
	if !ok {
		panic("value for keySession not found in context")
	}

	return value
}

func WithSession(ctx context.Context, value *ent.Session) context.Context {
	return context.WithValue(ctx, keySession, value)
}
