package models

import (
	"slices"
	"time"

	"github.com/google/uuid"

	"cameo/internal/config"
	"cameo/internal/ent"
	"cameo/internal/ent/schema_types/call"
)

type Call struct {
	ID               uuid.UUID             `json:"id"`
	Status           call.Status           `json:"status"`
	CallerID         uuid.UUID             `json:"caller_id"`
	CalleeID         uuid.UUID             `json:"callee_id"`
	StartedAt        *time.Time            `json:"started_at"`
	EndedAt          *time.Time            `json:"ended_at"`
	Title            *string               `json:"title"`
	Summary          *string               `json:"summary"`
	TranscriptStatus call.TranscriptStatus `json:"transcript_status"`
	HighlightCount   int                   `json:"highlight_count"`
	IsFavorite       bool                  `json:"is_favorite"`
	CreatedAt        time.Time             `json:"created_at"`
}

func FromCall(c *ent.Call, me uuid.UUID, highlightCount int) Call {
	return Call{
		ID:               c.ID,
		Status:           c.Status,
		CallerID:         c.CallerID,
		CalleeID:         c.CalleeID,
		StartedAt:        c.StartedAt,
		EndedAt:          c.EndedAt,
		Title:            c.Title,
		Summary:          c.Summary,
		TranscriptStatus: c.TranscriptStatus,
		HighlightCount:   highlightCount,
		IsFavorite:       slices.Contains(c.FavoritedBy, me),
		CreatedAt:        c.CreatedAt,
	}
}

type CallDetail struct {
	Call
	Highlights   []Highlight        `json:"highlights"`
	Transcript   []call.Segment     `json:"transcript"`
	RecordingURL *string            `json:"recording_url"`
	URLExpiresAt *time.Time         `json:"url_expires_at"`
	Photos       []Photo            `json:"photos"`
	ICEServers   []config.ICEServer `json:"ice_servers"`
}

type Highlight struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	OffsetSeconds float64   `json:"offset_seconds"`
	CreatedAt     time.Time `json:"created_at"`
}

func FromHighlight(h *ent.CallHighlight) Highlight {
	return Highlight{
		ID:            h.ID,
		UserID:        h.UserID,
		OffsetSeconds: h.OffsetSeconds,
		CreatedAt:     h.CreatedAt,
	}
}

// HighlightAdded is the signaling message telling the other participant about a new highlight.
type HighlightAdded struct {
	Type      string    `json:"type"`
	Highlight Highlight `json:"highlight"`
}

func NewHighlightAdded(h Highlight) HighlightAdded {
	return HighlightAdded{Type: "highlight_added", Highlight: h}
}

// PhotoShared is the signaling message telling the other participant about a photo shared during the call.
type PhotoShared struct {
	Type  string `json:"type"`
	Photo Photo  `json:"photo"`
}

func NewPhotoShared(p Photo) PhotoShared {
	return PhotoShared{Type: "photo_shared", Photo: p}
}
