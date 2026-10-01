package purchase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
)

const (
	subscribersURL = "https://api.revenuecat.com/v1/subscribers/"
	requestTimeout = 10 * time.Second
	errorBodyLimit = 512
)

type subscriber struct {
	Entitlements     map[string]entitlement       `json:"entitlements"`
	NonSubscriptions map[string][]nonSubscription `json:"non_subscriptions"`
}

type entitlement struct {
	// ExpiresDate is nil for a lifetime entitlement.
	ExpiresDate            *time.Time `json:"expires_date"`
	GracePeriodExpiresDate *time.Time `json:"grace_period_expires_date"`
}

// until is when the entitlement lapses, counting the billing grace period. A lifetime entitlement never lapses.
func (e entitlement) until() time.Time {
	if e.ExpiresDate == nil {
		return lifetime
	}
	if e.GracePeriodExpiresDate != nil && e.GracePeriodExpiresDate.After(*e.ExpiresDate) {
		return *e.GracePeriodExpiresDate
	}
	return *e.ExpiresDate
}

type nonSubscription struct {
	ID string `json:"id"`
}

// fetchSubscriber reads the RevenueCat customer whose app user id is userID, creating it when it does not exist yet.
func (s *Service) fetchSubscriber(ctx context.Context, userID uuid.UUID) (*subscriber, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, subscribersURL+url.PathEscape(userID.String()), nil)
	if err != nil {
		return nil, fmt.Errorf("create subscriber request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	request.Header.Set("Accept", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("get subscriber: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(response.Body, errorBodyLimit))
		return nil, fmt.Errorf("get subscriber: status %d: %s", response.StatusCode, body)
	}

	var decoded struct {
		Subscriber subscriber `json:"subscriber"`
	}
	if err = json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode subscriber: %w", err)
	}
	return &decoded.Subscriber, nil
}
