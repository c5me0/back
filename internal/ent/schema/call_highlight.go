package schema

import (
	"time"

	"github.com/google/uuid"
	"github.com/neko-sc/ent"
	"github.com/neko-sc/ent/schema/edge"
	"github.com/neko-sc/ent/schema/field"
	"github.com/neko-sc/ent/schema/index"
)

// CallHighlight holds the schema definition for the CallHighlight entity.
type CallHighlight struct {
	ent.Schema
}

// Fields of the CallHighlight.
func (CallHighlight) Fields() []ent.Field {
	return []ent.Field{
		field.UUID[uuid.UUID]("id").DefaultFunc(MustUUIDv7).Immutable(),

		field.Float("offset_seconds").Min(0),

		field.UUID[uuid.UUID]("call_id").Immutable(),
		field.UUID[uuid.UUID]("user_id").Immutable(),

		field.Time("created_at").DefaultFunc(time.Now).Immutable(),
	}
}

// Edges of the CallHighlight.
func (CallHighlight) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("call", Call.Type).
			Ref("highlights").
			Field("call_id").
			Unique().Required().Immutable(),
		edge.From("user", User.Type).
			Ref("call_highlights").
			Field("user_id").
			Unique().Required().Immutable(),
	}
}

// Indexes of the CallHighlight.
func (CallHighlight) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("call_id", "offset_seconds"),
	}
}
