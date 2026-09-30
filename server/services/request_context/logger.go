//revive:disable:var-naming // package name mirrors directory naming used across server
package request_context

import (
	"context"

	"github.com/rs/zerolog"
)

// Logger returns the request-scoped logger, or a no-op logger outside a request.
func Logger(ctx context.Context) zerolog.Logger {
	logger, ok := ctx.Value(keyLogger).(zerolog.Logger)
	if !ok {
		return zerolog.Nop()
	}

	return logger
}

func WithLogger(ctx context.Context, logger zerolog.Logger) context.Context {
	return context.WithValue(ctx, keyLogger, logger)
}
