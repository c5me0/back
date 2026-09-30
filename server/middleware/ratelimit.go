package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"

	"cameo/internal/protocol"
	"cameo/internal/tools/ro"
	"cameo/server/services/request_context"
)

const maxPhoneKeyBody = 4 << 10

var limitHandler = httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
	ro.WriteError(w, r, protocol.ErrorResponse{Code: protocol.RateLimit, Message: "too many requests"})
})

// RateLimitByIP limits requests per client IP. The IP comes from a chi ClientIPFrom* middleware when one is
// installed, otherwise from the connection's remote address.
func RateLimitByIP(requests int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.LimitBy(requests, window, clientIP, limitHandler)
}

// RateLimitByPhone limits requests per "phone" field of the JSON body, falling back to the client IP.
// It reads at most 4 KiB of the body and restores it for the handler.
func RateLimitByPhone(requests int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.LimitBy(requests, window, func(r *http.Request) (string, error) {
		body, err := io.ReadAll(io.LimitReader(r.Body, maxPhoneKeyBody))
		if err != nil {
			return "", err
		}
		r.Body = struct {
			io.Reader
			io.Closer
		}{io.MultiReader(bytes.NewReader(body), r.Body), r.Body}

		var payload struct {
			Phone string `json:"phone"`
		}
		if json.Unmarshal(body, &payload) != nil || payload.Phone == "" {
			return clientIP(r)
		}
		return "phone:" + payload.Phone, nil
	}, limitHandler)
}

// RateLimitByUser limits requests per authenticated user. It must run after the session middleware.
func RateLimitByUser(requests int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.LimitBy(requests, window, func(r *http.Request) (string, error) {
		return request_context.UserID(r.Context()).String(), nil
	}, limitHandler)
}

func clientIP(r *http.Request) (string, error) {
	if ip := chimiddleware.GetClientIP(r.Context()); ip != "" {
		return "ip:" + httprate.CanonicalizeIP(ip), nil
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + httprate.CanonicalizeIP(host), nil
}
