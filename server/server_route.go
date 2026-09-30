package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"cameo/internal/protocol"
	"cameo/internal/tools/ro"
	"cameo/server/handlers/probe"
	"cameo/server/middleware"
)

func (s *Server) route() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestInfo)
	r.Use(middleware.Logger(s.logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: s.config.Service.CORSAllowedOrigins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		ExposedHeaders: []string{"Request-ID"},
		MaxAge:         300,
	}))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		ro.WriteError(w, r, protocol.ErrorResponse{Code: protocol.NotFound, Message: "not found"})
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		ro.WriteJSON(w, http.StatusMethodNotAllowed, protocol.ErrorResponse{Code: protocol.InvalidRequest, Message: "method not allowed"})
	})

	probeHandler := probe.NewHandler(s.sqlDB, s.logger)
	r.Get("/livez", probeHandler.Live)
	r.Get("/readyz", probeHandler.Ready)

	r.Route("/v1", func(v1 chi.Router) {
		s.routeAuth(v1)

		v1.Group(func(authenticated chi.Router) {
			authenticated.Use(s.session.Middleware)

			s.routeAccount(authenticated)
			s.routeCouple(authenticated)
			s.routeDevice(authenticated)
			s.routePhoto(authenticated)
			s.routeCall(authenticated)
		})
	})

	return r
}
