package server

import (
	"github.com/go-chi/chi/v5"

	"cameo/internal/tools/ro"
	"cameo/server/handlers/v1/account"
)

func (s *Server) routeAccount(r chi.Router) {
	h := account.NewHandler(s.db)

	r.Get("/me", ro.HandleOut(h.Me))
	r.Patch("/me", ro.Handle(h.UpdateMe))
}
