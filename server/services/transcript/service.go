// Package transcript turns a finished call's recording into a mixed m4a, a transcript, a title and a summary.
//
// Input contract (written by the call service): {recording.dir}/{call_id}/ holds seg-{n}-{user_id}.ogg
// Opus files and meta.json = {"call_id", "started_at" (RFC 3339), "segments": [{"user_id", "file", "offset_ms"}]},
// where offset_ms is relative to started_at.
package transcript

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/rs/zerolog"

	"cameo/internal/config"
	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	"cameo/internal/ent/schema_types/call"
	"cameo/server/services/push"
	"cameo/server/services/storage"
)

const (
	processTimeout  = 30 * time.Minute
	shutdownTimeout = time.Minute
	dbTimeout       = 10 * time.Second
)

type Service struct {
	db      *ent.Client
	storage *storage.Service
	push    *push.Service
	cfg     *config.Config
	logger  zerolog.Logger
	// client is nil when OpenAI is not configured; recordings are then uploaded and the transcript is skipped.
	client *openai.Client

	sem chan struct{}
	wg  sync.WaitGroup
	// ctx is the parent of every pipeline run; Shutdown cancels it.
	ctx    context.Context
	cancel context.CancelFunc
}

func New(db *ent.Client, storage *storage.Service, push *push.Service, cfg *config.Config, logger zerolog.Logger) *Service {
	var client *openai.Client
	if cfg.OpenAI != nil {
		client = new(openai.NewClient(option.WithAPIKey(cfg.OpenAI.APIKey)))
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		db:      db,
		storage: storage,
		push:    push,
		cfg:     cfg,
		logger:  logger.With().Str("component", "transcript").Logger(),
		client:  client,
		sem:     make(chan struct{}, cfg.Recording.Concurrency),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Enqueue schedules the pipeline for a call whose transcript_status is pending (or processing, when resumed).
func (s *Service) Enqueue(callID uuid.UUID) {
	s.wg.Go(func() {
		select {
		case s.sem <- struct{}{}:
		case <-s.ctx.Done():
			return
		}
		defer func() { <-s.sem }()

		ctx, cancel := context.WithTimeout(s.ctx, processTimeout)
		defer cancel()
		s.process(ctx, callID)
	})
}

// Boot resumes calls left pending or processing by a previous process. Calls whose recording is gone are failed.
func (s *Service) Boot(ctx context.Context) error {
	ids, err := s.db.Call.Query().
		Where(entCall.TranscriptStatus.In(call.TranscriptStatusPending, call.TranscriptStatusProcessing)).
		IDs(ctx)
	if err != nil {
		return fmt.Errorf("query unfinished transcripts: %w", err)
	}

	for _, id := range ids {
		if _, err = os.Stat(filepath.Join(s.recordingDir(id), "meta.json")); err == nil {
			//nolint:contextcheck // pipelines run on the service context, not the boot context
			s.Enqueue(id)
			continue
		}

		s.logger.Warn().Err(err).Str("call_id", id.String()).Msg("recording of unfinished transcript is missing")
		err = s.db.Call.UpdateOneID(id).
			Where(entCall.TranscriptStatus.In(call.TranscriptStatusPending, call.TranscriptStatusProcessing)).
			Apply(ent.CallPatch{TranscriptStatus: ent.Some(call.TranscriptStatusFailed)}).
			Exec(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return fmt.Errorf("fail transcript of %s: %w", id, err)
		}
	}
	if len(ids) > 0 {
		s.logger.Info().Int("count", len(ids)).Msg("resumed unfinished transcripts")
	}

	return nil
}

// Shutdown interrupts running pipelines and waits for them. Interrupted calls stay processing and resume on the next Boot.
func (s *Service) Shutdown() {
	s.cancel()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(shutdownTimeout):
		s.logger.Warn().Msg("timed out waiting for transcript pipelines")
	}
}

func (s *Service) recordingDir(callID uuid.UUID) string {
	return filepath.Join(s.cfg.Recording.Dir, callID.String())
}
