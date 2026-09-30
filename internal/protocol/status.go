package protocol

import "net/http"

var statuses = map[string]int{
	InternalError:   http.StatusInternalServerError,
	InvalidRequest:  http.StatusBadRequest,
	Unauthenticated: http.StatusUnauthorized,
	Forbidden:       http.StatusForbidden,
	NotFound:        http.StatusNotFound,
	RateLimit:       http.StatusTooManyRequests,

	AuthInvalidCode: http.StatusBadRequest,

	CoupleNotConnected:       http.StatusNotFound,
	CoupleCodeNotFound:       http.StatusNotFound,
	CoupleSelf:               http.StatusBadRequest,
	CoupleAlreadyConnected:   http.StatusConflict,
	CouplePartnerUnavailable: http.StatusConflict,

	CallBusy:         http.StatusConflict,
	CallInvalidState: http.StatusConflict,

	PhotoUploadIncomplete: http.StatusConflict,
	PhotoInvalidState:     http.StatusConflict,

	PurchaseRequired: http.StatusPaymentRequired,
}

// HTTPStatus maps an error code to its HTTP status. Unknown codes are client errors.
func HTTPStatus(code string) int {
	if status, ok := statuses[code]; ok {
		return status
	}
	return http.StatusBadRequest
}
