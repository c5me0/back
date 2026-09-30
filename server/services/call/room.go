package call

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	"cameo/internal/ent/schema_types/call"
	entUser "cameo/internal/ent/user"
)

const (
	ringTimeout       = 60 * time.Second
	graceTimeout      = 30 * time.Second
	trackDrainTimeout = 2 * time.Second
	dbTimeout         = 10 * time.Second
)

// Room is a live call. It owns the call's status transitions until it finishes.
type Room struct {
	// ctx outlives the request that opened the room; timers and media callbacks use it.
	ctx        context.Context
	service    *Service
	id         uuid.UUID
	callerID   uuid.UUID
	calleeID   uuid.UUID
	callerName string
	dir        string
	logger     zerolog.Logger

	recording atomic.Bool
	tracks    sync.WaitGroup

	mu        sync.Mutex
	status    call.Status
	startedAt time.Time
	parts     [2]*Participant
	ringTimer *time.Timer
	segments  []segment
	finished  bool
}

// Open registers a room for a ringing call inserted by the caller, rings the callee through PushKit
// and marks the call missed if nobody answers within ringTimeout.
func (s *Service) Open(ctx context.Context, row *ent.Call, callerName string) {
	room := &Room{
		ctx:        context.WithoutCancel(ctx),
		service:    s,
		id:         row.ID,
		callerID:   row.CallerID,
		calleeID:   row.CalleeID,
		callerName: callerName,
		dir:        s.RecordingDir(row.ID),
		logger:     s.logger.With().Str("call_id", row.ID.String()).Logger(),
		status:     call.StatusRinging,
		parts:      [2]*Participant{{userID: row.CallerID}, {userID: row.CalleeID}},
	}
	room.ringTimer = time.AfterFunc(ringTimeout, func() {
		_ = room.finish(room.ctx, func(current call.Status) (call.Status, bool) {
			return call.StatusMissed, current == call.StatusRinging
		})
	})

	s.mu.Lock()
	s.rooms[row.ID] = room
	s.mu.Unlock()

	//nolint:contextcheck // push delivery runs in the background, detached from ctx
	s.push.NotifyUser(row.CalleeID, true, "", "", map[string]any{
		"type":        "incoming_call",
		"call_id":     row.ID,
		"caller_id":   row.CallerID,
		"caller_name": callerName,
	})
}

func (room *Room) participant(userID uuid.UUID) *Participant {
	for _, part := range room.parts {
		if part.userID == userID {
			return part
		}
	}
	return nil
}

func (room *Room) other(part *Participant) *Participant {
	if room.parts[0] == part {
		return room.parts[1]
	}
	return room.parts[0]
}

// attach makes sock the participant's signaling socket and returns the one it replaces.
func (room *Room) attach(part *Participant, sock *socket) (*socket, bool) {
	room.mu.Lock()
	defer room.mu.Unlock()

	if room.finished {
		return nil, false
	}
	previous := part.sock
	part.sock = sock
	room.updateGraceLocked(part)
	sock.trySend(room.stateLocked(part))
	return previous, true
}

// detach clears sock unless a newer socket already replaced it.
func (room *Room) detach(part *Participant, sock *socket) {
	room.mu.Lock()
	defer room.mu.Unlock()

	if part.sock != sock {
		return
	}
	part.sock = nil
	room.updateGraceLocked(part)
}

func (room *Room) notifyPeer(fromUserID uuid.UUID, msg any) {
	room.mu.Lock()
	defer room.mu.Unlock()

	for _, part := range room.parts {
		if part.userID != fromUserID && part.sock != nil {
			part.sock.trySend(msg)
		}
	}
}

func (room *Room) stateLocked(part *Participant) stateMessage {
	state := stateMessage{Type: "state", Status: room.status, PeerConnected: room.other(part).connected}
	if !room.startedAt.IsZero() {
		state.StartedAt = new(room.startedAt)
	}
	return state
}

func (room *Room) broadcastStateLocked() {
	for _, part := range room.parts {
		if part.sock != nil {
			part.sock.trySend(room.stateLocked(part))
		}
	}
}

// updateGraceLocked runs the participant's grace timer while an active call lacks its socket or media,
// ending the call when the participant does not come back within graceTimeout.
func (room *Room) updateGraceLocked(part *Participant) {
	needed := room.status == call.StatusActive && (part.sock == nil || !part.connected)
	switch {
	case needed && part.graceTimer == nil:
		var timer *time.Timer
		timer = time.AfterFunc(graceTimeout, func() {
			room.mu.Lock()
			current := part.graceTimer == timer
			room.mu.Unlock()
			if current {
				_ = room.finish(room.ctx, func(call.Status) (call.Status, bool) { return call.StatusEnded, true })
			}
		})
		part.graceTimer = timer
	case !needed && part.graceTimer != nil:
		part.graceTimer.Stop()
		part.graceTimer = nil
	}
}

// onConnectionState tracks the media state of the participant's current PeerConnection and
// activates the call once both participants are connected.
func (room *Room) onConnectionState(ctx context.Context, part *Participant, gen int, state webrtc.PeerConnectionState) {
	room.mu.Lock()
	if room.finished || part.gen != gen {
		room.mu.Unlock()
		return
	}

	switch state {
	case webrtc.PeerConnectionStateConnected:
		part.connected = true
	case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
		part.connected = false
	default:
		room.mu.Unlock()
		return
	}

	if room.status == call.StatusRinging && room.parts[0].connected && room.parts[1].connected {
		room.status = call.StatusActive
		room.startedAt = time.Now()
		room.ringTimer.Stop()
		startedAt := room.startedAt
		room.mu.Unlock()

		// Persist before announcing, so clients that see "active" can act on the call right away.
		updateCtx, cancel := context.WithTimeout(ctx, dbTimeout)
		err := room.service.db.Call.UpdateOneID(room.id).
			Where(entCall.Status.EQ(call.StatusRinging)).
			Apply(ent.CallPatch{Status: ent.Some(call.StatusActive), StartedAt: ent.Some(startedAt)}).
			Exec(updateCtx)
		cancel()
		if err != nil {
			room.logger.Error().Err(err).Msg("failed to activate call")
			_ = room.finish(ctx, func(call.Status) (call.Status, bool) { return call.StatusFailed, true })
			return
		}

		room.mu.Lock()
		if room.finished {
			room.mu.Unlock()
			return
		}
		room.recording.Store(true)
	}

	for _, p := range room.parts {
		room.updateGraceLocked(p)
	}
	room.broadcastStateLocked()
	room.mu.Unlock()
}

// finish ends the room with the status resolve picks from the current one; resolve rejects the transition
// by returning false. It tells both participants, closes their sockets and PeerConnections, persists the
// final status and hands the recording to the transcript pipeline. It completes even if ctx is canceled.
func (room *Room) finish(ctx context.Context, resolve func(current call.Status) (call.Status, bool)) error {
	room.mu.Lock()
	if room.finished {
		room.mu.Unlock()
		return errInvalidState
	}
	status, ok := resolve(room.status)
	if !ok {
		room.mu.Unlock()
		return errInvalidState
	}

	room.finished = true
	room.status = status
	room.recording.Store(false)
	room.ringTimer.Stop()
	neverActive := room.startedAt.IsZero()
	var connections []*webrtc.PeerConnection
	for _, part := range room.parts {
		if part.graceTimer != nil {
			part.graceTimer.Stop()
			part.graceTimer = nil
		}
		if part.sock != nil {
			part.sock.trySend(endedMessage{Type: "ended", Status: status})
			part.sock.shutdown(websocket.StatusNormalClosure, "call ended")
		}
		if part.pc != nil {
			connections = append(connections, part.pc)
		}
	}
	room.mu.Unlock()

	for _, pc := range connections {
		if err := pc.Close(); err != nil {
			room.logger.Warn().Err(err).Msg("failed to close peer connection")
		}
	}
	room.waitTracks()

	room.mu.Lock()
	segments := room.segments
	room.mu.Unlock()

	recorded := len(segments) > 0
	if recorded {
		if err := room.writeMeta(segments); err != nil {
			room.logger.Error().Err(err).Msg("failed to write recording metadata")
			recorded = false
		}
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dbTimeout)
	defer cancel()

	transcriptStatus := call.TranscriptStatusNone
	if recorded {
		transcriptStatus = call.TranscriptStatusPending
	}
	err := room.service.db.Call.UpdateOneID(room.id).
		Where(entCall.Status.In(call.StatusRinging, call.StatusActive)).
		Apply(ent.CallPatch{
			Status:           ent.Some(status),
			EndedAt:          ent.Some(time.Now()),
			TranscriptStatus: ent.Some(transcriptStatus),
		}).
		Exec(ctx)
	if err != nil {
		room.logger.Error().Err(err).Str("status", status.String()).Msg("failed to persist call end")
	}

	if recorded && err == nil {
		//nolint:contextcheck // the pipeline runs on the transcript service context and must outlive this call
		room.service.transcript.Enqueue(room.id)
	} else if err = os.RemoveAll(room.dir); err != nil {
		room.logger.Warn().Err(err).Msg("failed to remove recording")
	}

	room.service.mu.Lock()
	delete(room.service.rooms, room.id)
	room.service.mu.Unlock()

	room.logger.Info().Str("status", status.String()).Int("segments", len(segments)).Msg("call finished")

	if neverActive && status != call.StatusDeclined {
		//nolint:contextcheck // push delivery runs in the background, detached from ctx
		room.service.push.NotifyUser(room.calleeID, true, "", "", map[string]any{"type": "call_ended", "call_id": room.id})
	}
	if status == call.StatusMissed {
		room.notifyMissed(ctx)
	}
	return nil
}

func (room *Room) waitTracks() {
	done := make(chan struct{})
	go func() {
		room.tracks.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(trackDrainTimeout):
		room.logger.Warn().Msg("timed out waiting for track goroutines")
	}
}

func (room *Room) notifyMissed(ctx context.Context) {
	callee, err := room.service.db.User.Query().
		Where(entUser.ID.EQ(room.calleeID)).
		Columns(entUser.ID, entUser.CallAlert).
		Only(ctx)
	if err != nil {
		room.logger.Warn().Err(err).Msg("failed to load callee for missed call alert")
		return
	}
	if !callee.CallAlert {
		return
	}

	body := "받지 못한 전화가 있어요"
	if room.callerName != "" {
		body = room.callerName + "님의 전화를 받지 못했어요"
	}
	//nolint:contextcheck // push delivery runs in the background, detached from ctx
	room.service.push.NotifyUser(room.calleeID, false, "부재중 전화", body, map[string]any{"type": "missed_call", "call_id": room.id})
}
