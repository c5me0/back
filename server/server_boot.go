package server

import (
	"context"
	"fmt"
)

// boot runs startup work that needs every service, before the server accepts requests
// (e.g. sweeping calls left over from a previous process).
func (s *Server) boot(ctx context.Context) error {
	if err := s.call.Boot(ctx); err != nil {
		return fmt.Errorf("failed to boot call service: %w", err)
	}

	if err := s.transcript.Boot(ctx); err != nil {
		return fmt.Errorf("failed to boot transcript service: %w", err)
	}

	return nil
}
