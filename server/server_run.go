package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 60 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
	shutdownTimeout   = 15 * time.Second
)

func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer s.shutdown()
	defer cancel()

	if err := s.init(ctx); err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	if err := s.boot(ctx); err != nil {
		return fmt.Errorf("failed to boot server: %w", err)
	}

	return s.listen(ctx)
}

// listen serves until ctx is canceled, then drains in-flight requests.
func (s *Server) listen(ctx context.Context) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", s.http.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.logger.Info().Str("address", listener.Addr().String()).Msg("listening")

	served := make(chan error, 1)
	go func() {
		served <- s.http.Serve(listener)
	}()

	select {
	case err = <-served:
		return fmt.Errorf("failed to serve: %w", err)
	case <-ctx.Done():
	}

	s.logger.Info().Msg("shutting down http server")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	if err = s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shut down http server: %w", err)
	}
	if err = <-served; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}
