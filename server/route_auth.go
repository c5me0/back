package server

import (
	"time"

	"github.com/go-chi/chi/v5"

	"cameo/internal/tools/ro"
	"cameo/server/handlers/v1/auth"
	"cameo/server/middleware"
)

func (s *Server) routeAuth(r chi.Router) {
	h := auth.NewHandler(s.db, s.session, s.otp)

	r.With(middleware.RateLimitByIP(5, time.Minute), middleware.RateLimitByPhone(3, 10*time.Minute)).
		Post("/auth/phone/start", ro.HandleIn(h.PhoneStart))
	r.With(middleware.RateLimitByIP(10, time.Minute)).Post("/auth/phone/verify", ro.Handle(h.PhoneVerify))
	r.With(s.session.Middleware).Post("/auth/signout", ro.HandleNone(h.Signout))
}
