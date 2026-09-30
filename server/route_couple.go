package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"cameo/internal/tools/ro"
	"cameo/server/handlers/v1/couple"
	"cameo/server/middleware"
)

func (s *Server) routeCouple(r chi.Router) {
	h := couple.NewHandler(s.db, s.call, s.push, s.purchase)

	r.Get("/couple", ro.HandleOut(h.Get))
	r.With(middleware.RateLimitByUser(10, time.Minute)).
		Post("/couple", ro.Handle(h.Connect, ro.WithStatus(http.StatusCreated)))
	r.Delete("/couple", ro.HandleNone(h.Disconnect))
	r.Post("/couple/code", ro.HandleOut(h.RegenerateCode))
	r.With(s.purchase.Middleware).Post("/couple/restore", ro.HandleOut(h.Restore))
}
