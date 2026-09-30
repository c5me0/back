package call

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/oggwriter"
)

// segment is one continuous recording of a participant, starting offset_ms after the call started.
// A participant gets a new segment for every PeerConnection (reconnect).
type segment struct {
	UserID   uuid.UUID `json:"user_id"`
	File     string    `json:"file"`
	OffsetMS int64     `json:"offset_ms"`
}

// meta is meta.json, the recording contract consumed by the transcript pipeline.
type meta struct {
	CallID    uuid.UUID `json:"call_id"`
	StartedAt time.Time `json:"started_at"`
	Segments  []segment `json:"segments"`
}

// forward relays the participant's incoming audio to the other participant and records it while the call is active.
func (room *Room) forward(part *Participant, gen int, remote *webrtc.TrackRemote) {
	room.mu.Lock()
	if room.finished || part.gen != gen {
		room.mu.Unlock()
		return
	}
	room.tracks.Add(1)
	peer := room.other(part)
	room.mu.Unlock()
	defer room.tracks.Done()

	var writer *oggwriter.OggWriter
	defer func() {
		if writer != nil {
			if err := writer.Close(); err != nil {
				room.logger.Warn().Err(err).Msg("failed to close recording segment")
			}
		}
	}()

	recordingFailed := false
	for {
		packet, _, err := remote.ReadRTP()
		if err != nil {
			return
		}

		if out := peer.out.Load(); out != nil {
			if err = out.WriteRTP(packet); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				room.logger.Debug().Err(err).Msg("failed to relay rtp")
			}
		}

		if recordingFailed || !room.recording.Load() {
			continue
		}
		if writer == nil {
			if writer, err = room.openSegment(part.userID); err != nil {
				room.logger.Error().Err(err).Str("user_id", part.userID.String()).Msg("failed to open recording segment")
				recordingFailed = true
				continue
			}
		}
		if err = writer.WriteRTP(packet); err != nil {
			room.logger.Debug().Err(err).Msg("failed to record rtp")
		}
	}
}

// openSegment creates seg-{n}-{user_id}.ogg and records its offset from the call start.
func (room *Room) openSegment(userID uuid.UUID) (*oggwriter.OggWriter, error) {
	room.mu.Lock()
	defer room.mu.Unlock()

	if room.finished {
		return nil, errInvalidState
	}
	if err := os.MkdirAll(room.dir, 0o750); err != nil {
		return nil, fmt.Errorf("create recording directory: %w", err)
	}

	file := fmt.Sprintf("seg-%d-%s.ogg", len(room.segments), userID)
	writer, err := oggwriter.New(filepath.Join(room.dir, file), 48000, 2)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", file, err)
	}

	room.segments = append(room.segments, segment{
		UserID:   userID,
		File:     file,
		OffsetMS: time.Since(room.startedAt).Milliseconds(),
	})
	return writer, nil
}

func (room *Room) writeMeta(segments []segment) error {
	data, err := json.Marshal(meta{CallID: room.id, StartedAt: room.startedAt.UTC(), Segments: segments})
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	return os.WriteFile(filepath.Join(room.dir, "meta.json"), data, 0o600)
}
