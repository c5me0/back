package call

import "fmt"

type Status int8 //nolint:recvcheck // decoding requires a pointer receiver

// Append-only: values are a permanent DB/wire contract. Never renumber or reuse.
const (
	StatusRinging  Status = 0
	StatusActive   Status = 1
	StatusEnded    Status = 2
	StatusMissed   Status = 3
	StatusDeclined Status = 4
	StatusFailed   Status = 5
)

var statusNames = [...]string{
	"ringing",
	"active",
	"ended",
	"missed",
	"declined",
	"failed",
}

func (v Status) Validate() error {
	if v < 0 || int(v) >= len(statusNames) {
		return fmt.Errorf("invalid Status: %d", v)
	}
	return nil
}

func (v Status) String() string {
	if v.Validate() != nil {
		return fmt.Sprintf("Status(%d)", v)
	}
	return statusNames[v]
}

func (v Status) MarshalText() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return []byte(statusNames[v]), nil
}

func (v *Status) UnmarshalText(text []byte) error {
	for candidate := range Status(len(statusNames)) {
		if statusNames[candidate] == string(text) {
			*v = candidate
			return nil
		}
	}
	return fmt.Errorf("invalid Status: %q", text)
}
