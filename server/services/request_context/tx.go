//revive:disable:var-naming // package name mirrors directory naming used across server
package request_context

import (
	"context"
	"fmt"

	"cameo/internal/ent"
)

// Tx starts a transaction that the returned commit function commits on success or rolls back on error.
//
// Usage:
//
//	func (h *Handler) Handle(ctx context.Context, req *Request) (ret error) {
//	    tx, commit, err := request_context.Tx(ctx, h.db)
//	    if err != nil {
//	        return err
//	    }
//	    defer commit(&ret) // MUST use named return, NOT local err variable
//
//	    // use tx.Client() for queries...
//	}
func Tx(ctx context.Context, db *ent.Client) (*ent.Tx, func(*error), error) {
	tx, err := db.Tx(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	commit := func(errPtr *error) {
		if *errPtr != nil {
			_ = tx.Rollback()
			return
		}
		if commitErr := tx.Commit(); commitErr != nil {
			*errPtr = fmt.Errorf("failed to commit transaction: %w", commitErr)
		}
	}

	return tx, commit, nil
}
