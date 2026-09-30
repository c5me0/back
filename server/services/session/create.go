package session

import (
	"context"
	"fmt"
	"time"

	"cameo/internal/ent"
	"cameo/server/services/request_context"
)

// Create inserts a session for user and returns its signed bearer token.
func (s *Service) Create(ctx context.Context, tx *ent.Tx, user *ent.User) (token string, expiresAt time.Time, err error) {
	requestTime := request_context.Time(ctx)
	expiresAt = requestTime.Add(sessionValidity)

	session, err := tx.Session.Insert(ent.SessionInsert{
		UserID:    user.ID,
		CreatedAt: ent.Some(requestTime),
		UpdatedAt: ent.Some(requestTime),
		ExpiresAt: ent.Some(expiresAt),
	}).Save(ctx)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("insert session: %w", err)
	}

	token, err = s.signer.Sign(Session{
		Version: tokenVersion,
		ID:      session.ID,
		UserID:  session.UserID,
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign session: %w", err)
	}

	return token, expiresAt, nil
}
