package push

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"

	"cameo/internal/config"
)

type apnsProvider struct {
	client   *apns2.Client
	bundleID string
}

func newAPNS(cfg *config.APNS) (*apnsProvider, error) {
	key, err := token.AuthKeyFromFile(cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("load apns key: %w", err)
	}

	client := apns2.NewTokenClient(&token.Token{AuthKey: key, KeyID: cfg.KeyID, TeamID: cfg.TeamID})
	if cfg.Production {
		client = client.Production()
	} else {
		client = client.Development()
	}

	return &apnsProvider{client: client, bundleID: cfg.BundleID}, nil
}

func (p *apnsProvider) Send(ctx context.Context, n Notification) ([]string, error) {
	notification := &apns2.Notification{
		Topic:    p.bundleID,
		PushType: apns2.PushTypeAlert,
		Priority: apns2.PriorityHigh,
	}
	if n.VoIP {
		notification.Topic = p.bundleID + ".voip"
		notification.PushType = apns2.PushTypeVOIP
		notification.Payload = n.Data
	} else {
		alert := payload.NewPayload().AlertTitle(n.Title).AlertBody(n.Body).Sound("default")
		for key, value := range n.Data {
			alert.Custom(key, value)
		}
		notification.Payload = alert
	}

	var invalid []string
	var errs []error
	for _, deviceToken := range n.Tokens {
		notification.DeviceToken = deviceToken
		response, err := p.client.PushWithContext(ctx, notification)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("apns push: %w", err))
		case response.StatusCode == http.StatusGone ||
			response.Reason == apns2.ReasonBadDeviceToken ||
			response.Reason == apns2.ReasonUnregistered:
			invalid = append(invalid, deviceToken)
		case !response.Sent():
			errs = append(errs, fmt.Errorf("apns rejected notification: %d %s", response.StatusCode, response.Reason))
		}
	}

	return invalid, errors.Join(errs...)
}
