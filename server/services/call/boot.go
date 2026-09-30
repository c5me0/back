package call

import (
	"context"
	"fmt"
	"os"
	"time"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	"cameo/internal/ent/schema_types/call"
)

// Boot fails the calls a previous process left ringing or active, and removes their partial recordings.
func (s *Service) Boot(ctx context.Context) error {
	ids, err := s.db.Call.Query().
		Where(entCall.Status.In(call.StatusRinging, call.StatusActive)).
		IDs(ctx)
	if err != nil {
		return fmt.Errorf("query stale calls: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}

	err = s.db.Call.Update().
		Where(entCall.ID.In(ids...), entCall.Status.In(call.StatusRinging, call.StatusActive)).
		Apply(ent.CallPatch{Status: ent.Some(call.StatusFailed), EndedAt: ent.Some(time.Now())}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("fail stale calls: %w", err)
	}

	for _, id := range ids {
		if err = os.RemoveAll(s.RecordingDir(id)); err != nil {
			s.logger.Warn().Err(err).Str("call_id", id.String()).Msg("failed to remove stale recording")
		}
	}
	s.logger.Info().Int("count", len(ids)).Msg("failed stale calls")

	return nil
}
