package call

import (
	"fmt"
	"time"

	"github.com/pion/ice/v4"
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v4"

	"cameo/internal/config"
)

const (
	iceDisconnectedTimeout = 5 * time.Second
	iceFailedTimeout       = 15 * time.Second
	iceKeepAliveInterval   = 2 * time.Second
)

var opusCodec = webrtc.RTPCodecCapability{
	MimeType:    webrtc.MimeTypeOpus,
	ClockRate:   48000,
	Channels:    2,
	SDPFmtpLine: "minptime=10;useinbandfec=1",
}

// newAPI builds a WebRTC API that only negotiates Opus and serves every PeerConnection from one UDP port.
// The returned mux must be closed after every PeerConnection.
func newAPI(cfg *config.WebRTC) (*webrtc.API, *ice.MultiUDPMuxDefault, error) {
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: opusCodec, PayloadType: 111}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, nil, fmt.Errorf("register opus codec: %w", err)
	}

	registry := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, registry); err != nil {
		return nil, nil, fmt.Errorf("register interceptors: %w", err)
	}

	mux, err := ice.NewMultiUDPMuxFromPort(cfg.UDPPort, ice.UDPMuxFromPortWithNetworks(ice.NetworkTypeUDP4))
	if err != nil {
		return nil, nil, fmt.Errorf("listen on udp port %d: %w", cfg.UDPPort, err)
	}

	settings := webrtc.SettingEngine{}
	settings.SetICEUDPMux(mux)
	settings.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	settings.SetICETimeouts(iceDisconnectedTimeout, iceFailedTimeout, iceKeepAliveInterval)
	if len(cfg.PublicIPs) > 0 {
		err = settings.SetICEAddressRewriteRules(webrtc.ICEAddressRewriteRule{
			External:        cfg.PublicIPs,
			AsCandidateType: webrtc.ICECandidateTypeHost,
			Mode:            webrtc.ICEAddressRewriteReplace,
		})
		if err != nil {
			_ = mux.Close()
			return nil, nil, fmt.Errorf("set public ips: %w", err)
		}
	}

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(registry),
		webrtc.WithSettingEngine(settings),
	)
	return api, mux, nil
}
