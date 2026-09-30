// Package probe serves the liveness and readiness endpoints.
package probe

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// drainFile marks the instance as draining before a rollout stops it.
const drainFile = "/tmp/drain"

const probeTimeout = 2 * time.Second

type Handler struct {
	db     *sql.DB
	logger zerolog.Logger
}

func NewHandler(db *sql.DB, logger zerolog.Logger) *Handler {
	return &Handler{db: db, logger: logger}
}

// Live reports whether the process should keep running.
func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if draining() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Ready reports whether the process should receive traffic.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if draining() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), probeTimeout)
	defer cancel()

	if err := h.db.PingContext(ctx); err != nil {
		h.logger.Warn().Err(err).Str("component", "probe").Msg("database probe failed")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func draining() bool {
	_, err := os.Stat(drainFile)
	return err == nil
}
