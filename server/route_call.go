package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"cameo/internal/tools/ro"
	"cameo/server/handlers/v1/call"
)

func (s *Server) routeCall(r chi.Router) {
	h := call.NewHandler(s.db, s.storage, s.push, s.call, s.config.WebRTC.ICEServers)

	r.Post("/calls", ro.HandleOut(h.Create, ro.WithStatus(http.StatusCreated)))
	r.Get("/calls", ro.Handle(h.List))
	r.Get("/calls/{id}", ro.HandleOut(h.Get))
	r.Delete("/calls/{id}", ro.HandleNone(h.Delete))
	r.Post("/calls/{id}/decline", ro.HandleNone(h.Decline))
	r.Post("/calls/{id}/end", ro.HandleNone(h.End))
	r.Post("/calls/{id}/highlights", ro.Handle(h.CreateHighlight, ro.WithStatus(http.StatusCreated)))
	r.Put("/calls/{id}/favorite", ro.HandleNone(h.Favorite))
	r.Delete("/calls/{id}/favorite", ro.HandleNone(h.Unfavorite))
	r.Post("/calls/{id}/photos", ro.Handle(h.SharePhoto))
	r.Get("/calls/{id}/signal", h.Signal)
}
