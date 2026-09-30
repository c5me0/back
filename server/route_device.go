package server

import (
	"github.com/go-chi/chi/v5"

	"cameo/internal/tools/ro"
	"cameo/server/handlers/v1/device"
)

func (s *Server) routeDevice(r chi.Router) {
	h := device.NewHandler(s.db)

	r.Post("/devices", ro.HandleIn(h.Register))
	r.Delete("/devices/{token}", ro.HandleNone(h.Delete))
}
