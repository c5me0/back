// Package call runs live calls: an in-process SFU-lite relaying Opus between the two participants,
// WebSocket signaling, per-participant recording and the call status transitions of live calls.
package call

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog"

	"cameo/internal/config"
	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	"cameo/internal/ent/schema_types/call"
	"cameo/internal/protocol"
	"cameo/server/services/push"
	"cameo/server/services/transcript"
)

var errInvalidState = protocol.ErrorResponse{Code: protocol.CallInvalidState, Message: "call is not in a valid state for this action"}

type Service struct {
	db         *ent.Client
	api        *webrtc.API
	mux        *ice.MultiUDPMuxDefault
	cfg        *config.Config
	push       *push.Service
	transcript *transcript.Service
	logger     zerolog.Logger

	mu    sync.Mutex
	rooms map[uuid.UUID]*Room
}

func New(db *ent.Client, cfg *config.Config, push *push.Service, transcript *transcript.Service, logger zerolog.Logger) (*Service, error) {
	api, mux, err := newAPI(cfg.WebRTC)
	if err != nil {
		return nil, err
	}

	return &Service{
		db:         db,
		api:        api,
		mux:        mux,
		cfg:        cfg,
		push:       push,
		transcript: transcript,
		logger:     logger.With().Str("component", "call").Logger(),
		rooms:      make(map[uuid.UUID]*Room),
	}, nil
}

// RecordingDir is the local directory holding the call's recording segments and meta.json.
func (s *Service) RecordingDir(callID uuid.UUID) string {
	return filepath.Join(s.cfg.Recording.Dir, callID.String())
}

func (s *Service) room(callID uuid.UUID) *Room {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rooms[callID]
}

// HasRoom reports whether the call is live in this process.
func (s *Service) HasRoom(callID uuid.UUID) bool {
	return s.room(callID) != nil
}

// NotifyPeer sends msg over the signaling socket of the participant other than fromUserID, if connected.
func (s *Service) NotifyPeer(callID, fromUserID uuid.UUID, msg any) {
	if room := s.room(callID); room != nil {
		room.notifyPeer(fromUserID, msg)
	}
}

// Decline ends a ringing call on behalf of its callee.
func (s *Service) Decline(ctx context.Context, row *ent.Call, userID uuid.UUID) error {
	if userID != row.CalleeID {
		return protocol.ErrorResponse{Code: protocol.Forbidden, Message: "only the callee can decline the call"}
	}

	if room := s.room(row.ID); room != nil {
		return room.finish(ctx, func(current call.Status) (call.Status, bool) {
			return call.StatusDeclined, current == call.StatusRinging
		})
	}
	return s.transition(ctx, row.ID, call.StatusDeclined, call.StatusRinging)
}

// End hangs up a ringing or active call. A caller hanging up a ringing call makes it missed.
func (s *Service) End(ctx context.Context, row *ent.Call, userID uuid.UUID) error {
	if room := s.room(row.ID); room != nil {
		return room.finish(ctx, func(current call.Status) (call.Status, bool) {
			return endStatus(current, userID == row.CallerID), true
		})
	}
	if row.Status != call.StatusRinging && row.Status != call.StatusActive {
		return errInvalidState
	}
	return s.transition(ctx, row.ID, endStatus(row.Status, userID == row.CallerID), row.Status)
}

// ForceEnd ends the call if it is still live, e.g. when the couple disconnects.
func (s *Service) ForceEnd(ctx context.Context, row *ent.Call) error {
	if room := s.room(row.ID); room != nil {
		err := room.finish(ctx, func(call.Status) (call.Status, bool) { return call.StatusEnded, true })
		if errors.Is(err, errInvalidState) {
			return nil
		}
		return err
	}

	err := s.transition(ctx, row.ID, call.StatusEnded, call.StatusRinging, call.StatusActive)
	if errors.Is(err, errInvalidState) {
		return nil
	}
	return err
}

func endStatus(current call.Status, byCaller bool) call.Status {
	if current == call.StatusRinging && byCaller {
		return call.StatusMissed
	}
	return call.StatusEnded
}

// transition moves a call without a room (e.g. left over from another process) from one of the given statuses to status.
func (s *Service) transition(ctx context.Context, callID uuid.UUID, status call.Status, from ...call.Status) error {
	err := s.db.Call.UpdateOneID(callID).
		Where(entCall.Status.In(from...)).
		Apply(ent.CallPatch{Status: ent.Some(status), EndedAt: ent.Some(time.Now())}).
		Exec(ctx)
	if ent.IsNotFound(err) {
		return errInvalidState
	}
	if err != nil {
		return fmt.Errorf("update call status: %w", err)
	}
	return nil
}

// Shutdown fails every live call and releases the UDP port.
func (s *Service) Shutdown() {
	s.mu.Lock()
	rooms := make([]*Room, 0, len(s.rooms))
	for _, room := range s.rooms {
		rooms = append(rooms, room)
	}
	s.mu.Unlock()

	var wg sync.WaitGroup
	for _, room := range rooms {
		wg.Go(func() {
			_ = room.finish(room.ctx, func(call.Status) (call.Status, bool) { return call.StatusFailed, true })
		})
	}
	wg.Wait()

	if err := s.mux.Close(); err != nil {
		s.logger.Warn().Err(err).Msg("failed to close udp mux")
	}
}
