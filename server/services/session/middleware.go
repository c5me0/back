package session

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"cameo/internal/ent"
	entSession "cameo/internal/ent/session"
	"cameo/internal/protocol"
	"cameo/internal/tools/ro"
	"cameo/server/services/request_context"
)

var errInvalidToken = protocol.ErrorResponse{
	Code:    protocol.Unauthenticated,
	Message: "invalid token",
}

// Middleware authenticates the bearer token from the Authorization header, or the token query parameter
// for clients that cannot set headers (WebSocket). It stores the user ID and session in the request context.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := s.authenticate(r)
		if err != nil {
			ro.WriteError(w, r, err)
			return
		}

		ctx := request_context.WithSession(request_context.WithUserID(r.Context(), session.UserID), session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) authenticate(r *http.Request) (*ent.Session, error) {
	ctx := r.Context()
	now := request_context.Time(ctx)
	logger := request_context.Logger(ctx)

	token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		return nil, protocol.ErrorResponse{
			Code:    protocol.Unauthenticated,
			Message: "missing token",
		}
	}

	signed := &Session{}
	if err := s.verifier.VerifyAndUnmarshal(token, signed); err != nil {
		logger.Debug().Err(err).Msg("failed to verify session token")
		return nil, errInvalidToken
	}
	if signed.Version != tokenVersion {
		logger.Debug().Int("version", signed.Version).Msg("invalid session token version")
		return nil, errInvalidToken
	}

	session, err := s.db.Session.Get(ctx, signed.ID)
	if err != nil {
		if ent.IsNotFound(err) {
			logger.Debug().Str("session", signed.ID.String()).Msg("session not found")
			return nil, errInvalidToken
		}
		return nil, fmt.Errorf("query session: %w", err)
	}

	if session.ExpiresAt.Before(now) {
		logger.Debug().Str("session", signed.ID.String()).Msg("session expired")
		return nil, protocol.ErrorResponse{
			Code:    protocol.Unauthenticated,
			Message: "session expired",
		}
	}

	if session.UpdatedAt.Before(now.Add(-time.Hour)) {
		session, err = session.Update().
			Set(entSession.UpdatedAt, now).
			Set(entSession.ExpiresAt, now.Add(sessionValidity)).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("renew session: %w", err)
		}
	}

	return session, nil
}
