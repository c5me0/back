package transcript

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	entCallHighlight "cameo/internal/ent/callhighlight"
	"cameo/internal/ent/schema_types/call"
	entUser "cameo/internal/ent/user"
)

const audioContentType = "audio/mp4"

// meta is the part of meta.json, written by the call service, that the pipeline needs.
type meta struct {
	Segments []segment `json:"segments"`
}

type segment struct {
	UserID   uuid.UUID `json:"user_id"`
	File     string    `json:"file"`
	OffsetMS int64     `json:"offset_ms"`
}

// track is one participant's recording segments, in recording order.
type track struct {
	userID   uuid.UUID
	segments []segment
}

// process runs the pipeline and marks the call failed when it errors. A shutdown leaves the call processing, so Boot resumes it.
func (s *Service) process(ctx context.Context, callID uuid.UUID) {
	logger := s.logger.With().Str("call_id", callID.String()).Logger()
	startedAt := time.Now()

	err := s.run(ctx, callID, logger)
	if err == nil {
		logger.Info().Dur("elapsed", time.Since(startedAt)).Msg("transcript pipeline finished")
		return
	}
	if s.ctx.Err() != nil {
		logger.Warn().Err(err).Msg("transcript pipeline interrupted by shutdown")
		return
	}

	logger.Error().Err(err).Msg("transcript pipeline failed")
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dbTimeout)
	defer cancel()
	err = s.db.Call.UpdateOneID(callID).
		Where(entCall.TranscriptStatus.EQ(call.TranscriptStatusProcessing)).
		Apply(ent.CallPatch{TranscriptStatus: ent.Some(call.TranscriptStatusFailed)}).
		Exec(ctx)
	if err != nil && !ent.IsNotFound(err) {
		logger.Error().Err(err).Msg("failed to mark transcript failed")
	}
}

func (s *Service) run(ctx context.Context, callID uuid.UUID, logger zerolog.Logger) error {
	err := s.db.Call.UpdateOneID(callID).
		Where(entCall.TranscriptStatus.In(call.TranscriptStatusPending, call.TranscriptStatusProcessing)).
		Apply(ent.CallPatch{TranscriptStatus: ent.Some(call.TranscriptStatusProcessing)}).
		Exec(ctx)
	if ent.IsNotFound(err) {
		logger.Debug().Msg("call is gone or not pending, skipping transcript")
		return nil
	}
	if err != nil {
		return fmt.Errorf("claim call: %w", err)
	}

	row, err := s.db.Call.Get(ctx, callID)
	if ent.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load call: %w", err)
	}

	dir := s.recordingDir(callID)
	data, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return fmt.Errorf("read meta: %w", err)
	}
	var recording meta
	if err = json.Unmarshal(data, &recording); err != nil {
		return fmt.Errorf("parse meta: %w", err)
	}
	if len(recording.Segments) == 0 {
		return errors.New("meta has no segments")
	}

	var tracks []track
	for _, seg := range recording.Segments {
		index := slices.IndexFunc(tracks, func(t track) bool { return t.userID == seg.UserID })
		if index < 0 {
			tracks = append(tracks, track{userID: seg.UserID})
			index = len(tracks) - 1
		}
		tracks[index].segments = append(tracks[index].segments, seg)
	}

	userPaths := make([]string, 0, len(tracks))
	for _, t := range tracks {
		path := filepath.Join(dir, t.userID.String()+".m4a")
		if err = s.encodeTrack(ctx, dir, t.segments, path); err != nil {
			return fmt.Errorf("encode track of %s: %w", t.userID, err)
		}
		userPaths = append(userPaths, path)
	}

	// With a single participant the mixed recording is that participant's track.
	mixedPath := userPaths[0]
	if len(userPaths) > 1 {
		mixedPath = filepath.Join(dir, "mixed.m4a")
		if err = s.mix(ctx, userPaths, mixedPath); err != nil {
			return fmt.Errorf("mix tracks: %w", err)
		}
	}

	mixed, err := os.Stat(mixedPath)
	if err != nil {
		return fmt.Errorf("stat mixed recording: %w", err)
	}

	// Only the mixed recording is stored; the per-participant tracks stay local for transcription.
	prefix := fmt.Sprintf("calls/%s/%s/", row.CoupleID, row.ID)
	mixedKey := prefix + "mixed.m4a"
	if err = s.storage.FPut(ctx, mixedKey, mixedPath, audioContentType); err != nil {
		return fmt.Errorf("upload mixed recording: %w", err)
	}

	err = s.db.Call.UpdateOneID(callID).Apply(ent.CallPatch{
		RecordingKey:   ent.Some(mixedKey),
		RecordingBytes: ent.Some(mixed.Size()),
	}).Exec(ctx)
	if ent.IsNotFound(err) {
		s.discard(ctx, prefix, dir, logger)
		return nil
	}
	if err != nil {
		return fmt.Errorf("set recording key: %w", err)
	}
	logger.Info().Str("recording_key", mixedKey).Int64("recording_bytes", mixed.Size()).Int("tracks", len(tracks)).Msg("recording uploaded")

	if s.client == nil {
		err = s.db.Call.UpdateOneID(callID).Apply(ent.CallPatch{TranscriptStatus: ent.Some(call.TranscriptStatusSkipped)}).Exec(ctx)
		if err != nil && !ent.IsNotFound(err) {
			return fmt.Errorf("mark transcript skipped: %w", err)
		}
		logger.Info().Msg("openai is not configured, transcript skipped")
		removeDir(dir, logger)
		return nil
	}

	var segments []call.Segment
	for i, t := range tracks {
		transcribed, err := s.transcribe(ctx, userPaths[i], t.userID)
		if err != nil {
			return fmt.Errorf("transcribe track of %s: %w", t.userID, err)
		}
		segments = append(segments, transcribed...)
	}
	slices.SortStableFunc(segments, func(a, b call.Segment) int { return cmp.Compare(a.Start, b.Start) })

	users, err := s.db.User.Query().
		Where(entUser.ID.In(row.CallerID, row.CalleeID)).
		Columns(entUser.ID, entUser.DisplayName, entUser.HighlightAlert).
		All(ctx)
	if err != nil {
		return fmt.Errorf("load participants: %w", err)
	}

	patch := ent.CallPatch{Transcript: ent.Some(segments), TranscriptStatus: ent.Some(call.TranscriptStatusCompleted)}
	var result summary
	if len(segments) > 0 {
		highlights, err := s.db.CallHighlight.Query().
			Where(entCallHighlight.CallID.EQ(callID)).
			Columns(entCallHighlight.ID, entCallHighlight.OffsetSeconds).
			Order(entCallHighlight.OffsetSeconds.Asc()).
			All(ctx)
		if err != nil {
			return fmt.Errorf("load highlights: %w", err)
		}

		if result, err = s.summarize(ctx, row, users, segments, highlights); err != nil {
			return fmt.Errorf("summarize: %w", err)
		}
		patch.Title = ent.Some(result.Title)
		patch.Summary = ent.Some(result.Summary)
	}

	err = s.db.Call.UpdateOneID(callID).Apply(patch).Exec(ctx)
	if ent.IsNotFound(err) {
		s.discard(ctx, prefix, dir, logger)
		return nil
	}
	if err != nil {
		return fmt.Errorf("save transcript: %w", err)
	}

	for _, user := range users {
		if user.HighlightAlert {
			//nolint:contextcheck // fire-and-forget
			s.push.NotifyUser(user.ID, false, "통화 요약이 준비됐어요", result.Title, map[string]any{"type": "summary_ready", "call_id": callID})
		}
	}
	removeDir(dir, logger)

	return nil
}

// discard removes the uploaded objects and the local recording of a call deleted while the pipeline ran.
func (s *Service) discard(ctx context.Context, prefix, dir string, logger zerolog.Logger) {
	logger.Info().Msg("call was deleted during the transcript pipeline, discarding recording")
	if err := s.storage.RemovePrefix(ctx, prefix); err != nil {
		logger.Warn().Err(err).Msg("failed to remove uploaded recording")
	}
	removeDir(dir, logger)
}

func removeDir(dir string, logger zerolog.Logger) {
	if err := os.RemoveAll(dir); err != nil {
		logger.Warn().Err(err).Msg("failed to remove recording directory")
	}
}
