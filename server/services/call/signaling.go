package call

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/pion/webrtc/v4"

	"cameo/internal/ent/schema_types/call"
	"cameo/internal/protocol"
)

const gatherTimeout = 10 * time.Second

// clientMessage is any client-to-server frame: offer{sdp}, candidate{candidate, sdp_mid, sdp_mline_index} or hangup.
type clientMessage struct {
	Type          string  `json:"type"`
	SDP           string  `json:"sdp"`
	Candidate     string  `json:"candidate"`
	SDPMid        *string `json:"sdp_mid"`
	SDPMLineIndex *uint16 `json:"sdp_mline_index"`
}

type answerMessage struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

type stateMessage struct {
	Type          string      `json:"type"`
	Status        call.Status `json:"status"`
	StartedAt     *time.Time  `json:"started_at"`
	PeerConnected bool        `json:"peer_connected"`
}

type endedMessage struct {
	Type   string      `json:"type"`
	Status call.Status `json:"status"`
}

type errorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// read handles the socket's client frames until it closes.
func (room *Room) read(ctx context.Context, part *Participant, sock *socket) {
	for {
		var msg clientMessage
		if err := wsjson.Read(ctx, sock.conn, &msg); err != nil {
			if status := websocket.CloseStatus(err); status == -1 && !errors.Is(err, context.Canceled) {
				room.logger.Debug().Err(err).Msg("signaling socket closed")
			}
			return
		}

		switch msg.Type {
		case "offer":
			if err := room.negotiate(ctx, part, sock, msg.SDP); err != nil {
				room.logger.Warn().Err(err).Str("user_id", part.userID.String()).Msg("failed to negotiate")
				sock.trySend(errorMessage{Type: "error", Code: protocol.InvalidRequest, Message: "failed to negotiate the offer"})
			}
		case "candidate":
			room.addCandidate(part, webrtc.ICECandidateInit{
				Candidate:     msg.Candidate,
				SDPMid:        msg.SDPMid,
				SDPMLineIndex: msg.SDPMLineIndex,
			})
		case "hangup":
			_ = room.finish(ctx, func(current call.Status) (call.Status, bool) {
				return endStatus(current, part.userID == room.callerID), true
			})
		default:
			sock.trySend(errorMessage{Type: "error", Code: protocol.InvalidRequest, Message: fmt.Sprintf("unknown message type %q", msg.Type)})
		}
	}
}

// negotiate replaces the participant's PeerConnection with one answering offer and sends the answer
// once ICE gathering completes (the server does not trickle).
func (room *Room) negotiate(ctx context.Context, part *Participant, sock *socket, offer string) (ret error) {
	pc, err := room.service.api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return fmt.Errorf("create peer connection: %w", err)
	}
	defer func() {
		if ret != nil {
			_ = pc.Close()
		}
	}()

	room.mu.Lock()
	if room.finished {
		room.mu.Unlock()
		return errInvalidState
	}
	previous := part.pc
	part.gen++
	gen := part.gen
	part.pc = pc
	part.connected = false
	part.remoteDescribed = false
	room.updateGraceLocked(part)
	room.broadcastStateLocked()
	room.mu.Unlock()

	if previous != nil {
		if err = previous.Close(); err != nil {
			room.logger.Debug().Err(err).Msg("failed to close previous peer connection")
		}
	}

	track, err := webrtc.NewTrackLocalStaticRTP(opusCodec, "audio", "cameo")
	if err != nil {
		return fmt.Errorf("create track: %w", err)
	}
	sender, err := pc.AddTrack(track)
	if err != nil {
		return fmt.Errorf("add track: %w", err)
	}
	go func() {
		for {
			if _, _, err := sender.ReadRTCP(); err != nil {
				return
			}
		}
	}()
	part.out.Store(track)

	// The PeerConnection outlives the socket that negotiated it.
	detached := context.WithoutCancel(ctx)
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		room.onConnectionState(detached, part, gen, state)
	})
	pc.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		room.forward(part, gen, remote)
	})

	if err = pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
		return fmt.Errorf("set remote description: %w", err)
	}

	room.mu.Lock()
	var pending []webrtc.ICECandidateInit
	if part.gen == gen {
		part.remoteDescribed = true
		pending, part.pendingCandidates = part.pendingCandidates, nil
	}
	room.mu.Unlock()
	for _, candidate := range pending {
		if err = pc.AddICECandidate(candidate); err != nil {
			room.logger.Debug().Err(err).Msg("failed to add queued ice candidate")
		}
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("create answer: %w", err)
	}
	gathered := webrtc.GatheringCompletePromise(pc)
	if err = pc.SetLocalDescription(answer); err != nil {
		return fmt.Errorf("set local description: %w", err)
	}

	select {
	case <-gathered:
	case <-time.After(gatherTimeout):
		return errors.New("ice gathering timed out")
	}

	sock.trySend(answerMessage{Type: "answer", SDP: pc.LocalDescription().SDP})
	return nil
}

// addCandidate applies a remote candidate, queueing it until the current offer is applied.
func (room *Room) addCandidate(part *Participant, candidate webrtc.ICECandidateInit) {
	if candidate.Candidate == "" {
		return
	}

	room.mu.Lock()
	if !part.remoteDescribed {
		part.pendingCandidates = append(part.pendingCandidates, candidate)
		room.mu.Unlock()
		return
	}
	pc := part.pc
	room.mu.Unlock()

	if err := pc.AddICECandidate(candidate); err != nil {
		room.logger.Debug().Err(err).Msg("failed to add ice candidate")
	}
}
