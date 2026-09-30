package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"cameo/internal/tools/ro"
	"cameo/server/services/request_context"
)

// RequestInfo assigns the request ID and time, exposes the ID as the Request-ID header and turns panics into 500 responses.
func RequestInfo(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.Must(uuid.NewV7())
		r = r.WithContext(request_context.WithRequestID(request_context.WithTime(r.Context(), time.Now()), requestID))
		w.Header().Set("Request-ID", requestID.String())

		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(recovered)
			}
			ro.WriteError(w, r, fmt.Errorf("panic: %v", recovered))
		}()

		next.ServeHTTP(w, r)
	})
}
