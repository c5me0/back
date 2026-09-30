package device

import "fmt"

type Platform int8 //nolint:recvcheck // decoding requires a pointer receiver

// Append-only: values are a permanent DB/wire contract. Never renumber or reuse.
const (
	PlatformAPNS     Platform = 0
	PlatformAPNSVoIP Platform = 1
)

var platformNames = [...]string{
	"apns",
	"apns_voip",
}

func (v Platform) Validate() error {
	if v < 0 || int(v) >= len(platformNames) {
		return fmt.Errorf("invalid Platform: %d", v)
	}
	return nil
}

func (v Platform) String() string {
	if v.Validate() != nil {
		return fmt.Sprintf("Platform(%d)", v)
	}
	return platformNames[v]
}

func (v Platform) MarshalText() ([]byte, error) {
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return []byte(platformNames[v]), nil
}

func (v *Platform) UnmarshalText(text []byte) error {
	for candidate := range Platform(len(platformNames)) {
		if platformNames[candidate] == string(text) {
			*v = candidate
			return nil
		}
	}
	return fmt.Errorf("invalid Platform: %q", text)
}
