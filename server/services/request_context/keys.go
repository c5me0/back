//revive:disable:var-naming // package name mirrors directory naming used across server
package request_context

type contextKey uint32

const (
	keyTime contextKey = iota
	keyRequestID
	keyLogger
	keyUserID
	keySession
)
