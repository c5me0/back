package config

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/go-ozzo/ozzo-validation/v4/is"
)

type WebRTC struct {
	UDPPort    int         `json:"udp_port"`
	PublicIPs  []string    `json:"public_ips"`
	ICEServers []ICEServer `json:"ice_servers"`
}

type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

func (s ICEServer) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.URLs, validation.Required),
	)
}

func (w *WebRTC) Validate() error {
	if w.ICEServers == nil {
		w.ICEServers = []ICEServer{}
	}

	return validation.ValidateStruct(w,
		validation.Field(&w.UDPPort, validation.Required, validation.Min(1), validation.Max(65535)),
		validation.Field(&w.PublicIPs, validation.Each(is.IP)),
		validation.Field(&w.ICEServers),
	)
}
