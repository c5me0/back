package call

import "github.com/google/uuid"

// Segment is one transcribed utterance of a call, timed from the start of the call recording.
type Segment struct {
	SpeakerUserID uuid.UUID `json:"speaker_user_id"`
	Start         float64   `json:"start"`
	End           float64   `json:"end"`
	Text          string    `json:"text"`
}
