package photo

import "fmt"

type Status int8 //nolint:recvcheck // decoding requires a pointer receiver

// Append-only: values are a permanent DB/wire contract. Never renumber or reuse.
const (
	StatusPending  Status = 0
	StatusUploaded Status = 1
)

var statusNames = [...]string{
	"pending",
	"uploaded",
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
