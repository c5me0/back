package server

import "github.com/go-chi/chi/v5"

// routeWebhook registers the RevenueCat webhook only when deliveries can be verified.
func (s *Server) routeWebhook(r chi.Router) {
	if !s.purchase.Enabled() || s.config.RevenueCat.WebhookSecret == "" {
		return
	}

	r.Post("/webhooks/revenuecat", s.purchase.Webhook)
}
