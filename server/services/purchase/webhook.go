package purchase

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	entUser "cameo/internal/ent/user"
	"cameo/internal/protocol"
	"cameo/internal/tools/ro"
)

const (
	webhookBodyLimit   = 1 << 20
	signatureTolerance = 5 * time.Minute
)

type webhookEvent struct {
	Type              string   `json:"type"`
	AppUserID         string   `json:"app_user_id"`
	OriginalAppUserID string   `json:"original_app_user_id"`
	Aliases           []string `json:"aliases"`
	TransferredTo     []string `json:"transferred_to"`
	TransferredFrom   []string `json:"transferred_from"`
}

// Webhook re-syncs every user a RevenueCat event mentions. Any non-200 response makes RevenueCat retry the delivery.
func (s *Service) Webhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, webhookBodyLimit))
	if err != nil {
		ro.WriteError(w, r, protocol.ErrorResponse{Code: protocol.InvalidRequest, Message: fmt.Sprintf("failed to read body: %v", err)})
		return
	}
	if !s.verifySignature(r.Header.Get("X-Revenuecat-Webhook-Signature"), body) {
		ro.WriteError(w, r, protocol.ErrorResponse{Code: protocol.Unauthenticated, Message: "invalid webhook signature"})
		return
	}

	var payload struct {
		Event webhookEvent `json:"event"`
	}
	if err = json.Unmarshal(body, &payload); err != nil {
		ro.WriteError(w, r, protocol.ErrorResponse{Code: protocol.InvalidRequest, Message: fmt.Sprintf("failed to parse event: %v", err)})
		return
	}
	event := payload.Event
	if event.Type == "TEST" {
		ro.WriteJSON(w, http.StatusOK, struct{}{})
		return
	}

	// App user ids that are not ours (RevenueCat anonymous ids) never match a user.
	var candidates []uuid.UUID
	for _, appUserID := range slices.Concat([]string{event.AppUserID, event.OriginalAppUserID}, event.Aliases, event.TransferredTo, event.TransferredFrom) {
		if userID, err := uuid.Parse(appUserID); err == nil {
			candidates = append(candidates, userID)
		}
	}
	userIDs, err := s.db.User.Query().Where(entUser.ID.In(candidates...)).IDs(ctx)
	if err != nil {
		ro.WriteError(w, r, fmt.Errorf("load users: %w", err))
		return
	}

	for _, userID := range userIDs {
		if _, err = s.Sync(ctx, userID); err != nil {
			ro.WriteError(w, r, fmt.Errorf("sync user %s: %w", userID, err))
			return
		}
	}

	s.logger.Info().Str("type", event.Type).Int("users", len(userIDs)).Msg("synced purchases from webhook")
	ro.WriteJSON(w, http.StatusOK, struct{}{})
}

// verifySignature checks a "t=<unix seconds>,v1=<hex>[,v1=<hex>...]" header against HMAC-SHA256(secret, "<t>.<body>").
// Several v1 entries appear while the secret is being rotated; any match within the tolerance is accepted.
func (s *Service) verifySignature(header string, body []byte) bool {
	var timestamp string
	var signatures [][]byte
	for part := range strings.SplitSeq(header, ",") {
		key, value, _ := strings.Cut(strings.TrimSpace(part), "=")
		switch key {
		case "t":
			timestamp = value
		case "v1":
			if signature, err := hex.DecodeString(value); err == nil {
				signatures = append(signatures, signature)
			}
		}
	}

	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(seconds, 0)).Abs() > signatureTolerance {
		return false
	}

	mac := hmac.New(sha256.New, []byte(s.config.WebhookSecret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	expected := mac.Sum(nil)
	return slices.ContainsFunc(signatures, func(signature []byte) bool { return hmac.Equal(signature, expected) })
}
