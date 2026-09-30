//revive:disable:var-naming // package name mirrors directory naming used across server
package request_context

import (
	"context"
	"time"
)

// Time returns the time the request was received.
func Time(ctx context.Context) time.Time {
	value, ok := ctx.Value(keyTime).(time.Time)
	if !ok {
		return time.Time{}
	}

	return value
}

func WithTime(ctx context.Context, value time.Time) context.Context {
	return context.WithValue(ctx, keyTime, value)
}
