package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"cameo/internal/tools/ro"
	"cameo/server/handlers/v1/photo"
)

func (s *Server) routePhoto(r chi.Router) {
	h := photo.NewHandler(s.db, s.storage)

	r.Post("/photos/upload-url", ro.Handle(h.UploadURL, ro.WithStatus(http.StatusCreated)))
	r.Post("/photos/{id}/complete", ro.HandleOut(h.Complete))
	r.Get("/photos", ro.Handle(h.List))
	r.Get("/photos/{id}", ro.HandleOut(h.Get))
	r.Put("/photos/{id}/favorite", ro.HandleNone(h.Favorite))
	r.Delete("/photos/{id}/favorite", ro.HandleNone(h.Unfavorite))
	r.Delete("/photos/{id}", ro.HandleNone(h.Delete))
}
