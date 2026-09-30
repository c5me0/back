package protocol

import "fmt"

// ErrorResponse is the client-facing error envelope with a code, message, and optional metadata.
//
//nolint:errname // ErrorResponse is a convention for error responses
type ErrorResponse struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Meta    map[string]any `json:"meta,omitempty"`
}

func (r ErrorResponse) Error() string {
	return fmt.Sprintf("%s: %s", r.Code, r.Message)
}

// Is matches any ErrorResponse with the same code.
func (r ErrorResponse) Is(target error) bool {
	t, ok := target.(ErrorResponse)
	return ok && r.Code == t.Code
}
