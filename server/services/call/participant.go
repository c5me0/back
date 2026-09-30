package call

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"

	"cameo/internal/protocol"
)

const (
	sendBuffer   = 32
	pingInterval = 15 * time.Second
	writeTimeout = 10 * time.Second

	// closeReplaced closes a participant's previous socket when the same user connects again.
	closeReplaced websocket.StatusCode = 4000
)

// Participant is one side of a room. It outlives signaling sockets and PeerConnections, which reconnects replace.
type Participant struct {
	userID uuid.UUID
	// out relays the other participant's audio to this participant.
	out atomic.Pointer[webrtc.TrackLocalStaticRTP]

	// The fields below are guarded by Room.mu.
	sock *socket
	pc   *webrtc.PeerConnection
	// gen identifies the current PeerConnection; callbacks of replaced ones are ignored.
	gen               int
	remoteDescribed   bool
	pendingCandidates []webrtc.ICECandidateInit
	connected         bool
	graceTimer        *time.Timer
}

// socket is one signaling WebSocket. Only its writer goroutine writes to the connection.
type socket struct {
	conn      *websocket.Conn
	send      chan any
	quit      chan struct{}
	closeOnce sync.Once
	code      websocket.StatusCode
	reason    string
}

func newSocket(conn *websocket.Conn) *socket {
	return &socket{conn: conn, send: make(chan any, sendBuffer), quit: make(chan struct{})}
}

// trySend queues msg without blocking; it is dropped when the queue is full.
func (s *socket) trySend(msg any) {
	select {
	case s.send <- msg:
	default:
	}
}

// shutdown makes the writer flush queued messages and close the connection with code.
func (s *socket) shutdown(code websocket.StatusCode, reason string) {
	s.closeOnce.Do(func() {
		s.code, s.reason = code, reason
		close(s.quit)
	})
}

func (s *socket) write(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case msg := <-s.send:
			if err := s.writeJSON(ctx, msg); err != nil {
				_ = s.conn.CloseNow()
				return
			}
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := s.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				_ = s.conn.CloseNow()
				return
			}
		case <-s.quit:
			for {
				select {
				case msg := <-s.send:
					if err := s.writeJSON(ctx, msg); err != nil {
						_ = s.conn.CloseNow()
						return
					}
				default:
					_ = s.conn.Close(s.code, s.reason)
					return
				}
			}
		}
	}
}

func (s *socket) writeJSON(ctx context.Context, msg any) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return wsjson.Write(ctx, s.conn, msg)
}

// Join serves the user's signaling socket for a live call and blocks until the socket closes.
func (s *Service) Join(ctx context.Context, callID, userID uuid.UUID, conn *websocket.Conn) error {
	room := s.room(callID)
	if room == nil {
		_ = conn.Close(websocket.StatusNormalClosure, "call is not live")
		return errInvalidState
	}
	part := room.participant(userID)
	if part == nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "not a participant")
		return protocol.ErrorResponse{Code: protocol.Forbidden, Message: "not a participant of the call"}
	}

	sock := newSocket(conn)
	previous, ok := room.attach(part, sock)
	if !ok {
		_ = conn.Close(websocket.StatusNormalClosure, "call is not live")
		return errInvalidState
	}
	if previous != nil {
		previous.shutdown(closeReplaced, "replaced")
	}

	var writer sync.WaitGroup
	writer.Go(func() { sock.write(ctx) })

	room.read(ctx, part, sock)

	room.detach(part, sock)
	sock.shutdown(websocket.StatusNormalClosure, "")
	writer.Wait()
	return nil
}
