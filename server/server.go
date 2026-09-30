// Package server wires configuration, services and routes into the HTTP server.
package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/rs/zerolog"

	"cameo/internal/config"
	"cameo/internal/ent"
	"cameo/server/services/call"
	"cameo/server/services/otp"
	"cameo/server/services/push"
	"cameo/server/services/session"
	"cameo/server/services/storage"
	"cameo/server/services/transcript"
)

type Server struct {
	config *config.Config
	logger zerolog.Logger

	db    *ent.Client
	sqlDB *sql.DB

	session *session.Service
	otp     otp.Provider
	storage *storage.Service
	push    *push.Service

	transcript *transcript.Service
	call       *call.Service

	http *http.Server
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) init(ctx context.Context) error {
	var err error

	s.config, err = config.LoadConfig()
	if err != nil {
		return err
	}

	s.logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "2006-01-02 15:04:05 MST",
	}).Level(s.config.Level()).With().Str("version", config.Version).Timestamp().Logger()

	signer, err := s.config.Token.Signer()
	if err != nil {
		return fmt.Errorf("failed to create signer: %w", err)
	}

	verifier, err := s.config.Token.Verifier()
	if err != nil {
		return fmt.Errorf("failed to create verifier: %w", err)
	}

	if err = s.setupDatabase(ctx); err != nil {
		return fmt.Errorf("failed to setup database: %w", err)
	}

	s.session = session.NewService(s.db, signer, verifier)

	s.otp, err = otp.New(s.config, s.logger)
	if err != nil {
		return fmt.Errorf("failed to create otp provider: %w", err)
	}

	s.storage, err = storage.New(ctx, s.config.Storage)
	if err != nil {
		return fmt.Errorf("failed to create storage service: %w", err)
	}

	s.push, err = push.New(s.config, s.db, s.logger)
	if err != nil {
		return fmt.Errorf("failed to create push service: %w", err)
	}

	s.transcript = transcript.New(s.db, s.storage, s.push, s.config, s.logger)

	s.call, err = call.New(s.db, s.config, s.push, s.transcript, s.logger)
	if err != nil {
		return fmt.Errorf("failed to create call service: %w", err)
	}

	s.http = &http.Server{
		Addr:              config.ListenAddress(),
		Handler:           s.route(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	return nil
}

func (s *Server) shutdown() {
	if s.call != nil {
		s.logger.Info().Str("component", "call").Msg("ending live calls")
		s.call.Shutdown()
	}

	if s.transcript != nil {
		s.logger.Info().Str("component", "transcript").Msg("waiting for transcript pipelines")
		s.transcript.Shutdown()
	}

	if s.push != nil {
		s.logger.Info().Str("component", "push").Msg("waiting for pending push notifications")
		s.push.Shutdown()
	}

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			s.logger.Error().Err(err).Msg("failed to close database client")
		} else {
			s.logger.Info().Str("component", "database").Msg("database client closed")
		}
	}
}
