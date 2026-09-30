// Package push delivers notifications to users' registered devices.
package push

import "context"

// Notification is one message delivered to every token in Tokens.
// VoIP notifications carry only Data (PushKit); alert notifications show Title and Body with Data as custom keys.
type Notification struct {
	Tokens []string
	VoIP   bool
	Title  string
	Body   string
	Data   map[string]any
}

type Provider interface {
	// Send delivers n and returns the tokens the push service reported as permanently invalid.
	Send(ctx context.Context, n Notification) (invalidTokens []string, err error)
}
