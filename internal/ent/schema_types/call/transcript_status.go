package call

import "fmt"

type TranscriptStatus int8 //nolint:recvcheck // decoding requires a pointer receiver

// Append-only: values are a permanent DB/wire contract. Never renumber or reuse.
const (
	TranscriptStatusNone       TranscriptStatus = 0
	TranscriptStatusPending    TranscriptStatus = 1
	TranscriptStatusProcessing TranscriptStatus = 2
	TranscriptStatusCompleted  TranscriptStatus = 3
	TranscriptStatusFailed     TranscriptStatus = 4
	TranscriptStatusSkipped    TranscriptStatus = 5
)

var transcriptStatusNames = [...]string{
	"none",
	"pending",
	"processing",
	"completed",
	"failed",
	"skipped",
}

func (v TranscriptStatus) Validate() error {
	if v < 0 || int(v) >= len(transcriptStatusNames) {
		return fmt.Errorf("invalid TranscriptStatus: %d", v)
	}
	return nil
}

func (v TranscriptStatus) String() string {
	if v.Validate() != nil {
		return fmt.Sprintf("TranscriptStatus(%d)", v)
	}
	return transcriptStatusNames[v]
}

func (v TranscriptStatus) MarshalText() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return []byte(transcriptStatusNames[v]), nil
}

func (v *TranscriptStatus) UnmarshalText(text []byte) error {
	for candidate := range TranscriptStatus(len(transcriptStatusNames)) {
		if transcriptStatusNames[candidate] == string(text) {
			*v = candidate
			return nil
		}
	}
	return fmt.Errorf("invalid TranscriptStatus: %q", text)
}
