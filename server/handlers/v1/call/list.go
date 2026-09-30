package call

import (
	"context"
	"fmt"

	"cameo/internal/ent"
	entCall "cameo/internal/ent/call"
	"cameo/server/handlers/v1/models"
)

type ListRequest struct {
	models.Query
	Favorite bool `query:"favorite"`
}

// List returns the couple's calls, newest first.
func (h *Handler) List(ctx context.Context, req *ListRequest) (*models.Page[models.Call], error) {
	user, err := currentUser(ctx, h.db)
	if err != nil {
		return nil, err
	}

	query := h.db.Call.Query().
		Where(entCall.CoupleID.EQ(*user.CoupleID)).
		Columns(entCall.ID, entCall.Status, entCall.TranscriptStatus, entCall.Title, entCall.Summary, entCall.FavoritedBy,
			entCall.StartedAt, entCall.EndedAt, entCall.CallerID, entCall.CalleeID, entCall.CreatedAt).
		WithCount(entCall.Highlights)
	if req.Favorite {
		query.Where(entCall.FavoritedBy.Contains(user.ID))
	}

	rows, err := query.Modify(req.Modify).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list calls: %w", err)
	}

	page := models.Paginate(rows, req.PageSize(), func(row *ent.Call) models.Call {
		count, _ := row.Edges.Count(entCall.Highlights)
		return models.FromCall(row, user.ID, count)
	}, func(row *ent.Call) models.Cursor {
		return models.Cursor{Time: row.CreatedAt, ID: row.ID}
	})
	return &page, nil
}
