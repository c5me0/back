package protocol

// Common errors
const (
	InternalError   = "internal_error"
	InvalidRequest  = "invalid_request"
	Unauthenticated = "unauthenticated"
	Forbidden       = "forbidden"
	NotFound        = "not_found"
	RateLimit       = "rate_limit"
)

// Auth errors
const (
	AuthInvalidCode = "auth:invalid_code"
)

// Couple errors
const (
	CoupleNotConnected       = "couple:not_connected"
	CoupleCodeNotFound       = "couple:code_not_found"
	CoupleSelf               = "couple:self"
	CoupleAlreadyConnected   = "couple:already_connected"
	CouplePartnerUnavailable = "couple:partner_unavailable"
)

// Call errors
const (
	CallBusy         = "call:busy"
	CallInvalidState = "call:invalid_state"
)

// Photo errors
const (
	PhotoUploadIncomplete = "photo:upload_incomplete"
	PhotoInvalidState     = "photo:invalid_state"
)

// Purchase errors
const (
	PurchaseRequired = "purchase:required"
)
