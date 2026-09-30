package server

import (
	"time"

	"github.com/go-chi/chi/v5"

	"cameo/internal/tools/ro"
	"cameo/server/handlers/v1/account"
	"cameo/server/middleware"
)

func (s *Server) routeAccount(r chi.Router) {
	h := account.NewHandler(s.db, s.purchase)

	r.Get("/me", ro.HandleOut(h.Me))
	r.Patch("/me", ro.Handle(h.UpdateMe))
	r.With(middleware.RateLimitByUser(10, time.Minute)).Post("/me/purchases/sync", ro.HandleOut(h.SyncPurchases))
}
