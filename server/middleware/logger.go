package middleware

import (
	"errors"
	"net/http"
	"runtime/debug"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"cameo/server/services/request_context"
)

// Logger stores a request-scoped logger in the context and logs every completed request.
// Panics are logged with their stack and re-raised for RequestInfo to answer.
func Logger(base zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := base.With().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("request_id", request_context.RequestID(r.Context()).String()).
				Logger()
			wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				if recovered := recover(); recovered != nil {
					if err, ok := recovered.(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
						logger.Error().Interface("panic", recovered).Bytes("stack", debug.Stack()).Msg("handler panicked")
					}
					panic(recovered)
				}

				status := wrapped.Status()
				if status == 0 {
					status = http.StatusOK
				}
				level := zerolog.DebugLevel
				switch {
				case status >= http.StatusInternalServerError:
					level = zerolog.ErrorLevel
				case status >= http.StatusBadRequest:
					level = zerolog.InfoLevel
				}
				logger.WithLevel(level).Int("status", status).Dur("duration", time.Since(request_context.Time(r.Context()))).Msg("request completed")
			}()

			next.ServeHTTP(wrapped, r.WithContext(request_context.WithLogger(r.Context(), logger)))
		})
	}
}
