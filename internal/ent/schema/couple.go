package schema

import (
	"time"

	"github.com/google/uuid"
	"github.com/neko-sc/ent"
	"github.com/neko-sc/ent/schema/edge"
	"github.com/neko-sc/ent/schema/field"
)

// Couple holds the schema definition for the Couple entity.
type Couple struct {
	ent.Schema
}

// Fields of the Couple.
func (Couple) Fields() []ent.Field {
	return []ent.Field{
		field.UUID[uuid.UUID]("id").DefaultFunc(MustUUIDv7).Immutable(),

		field.Time("disconnected_at").Optional().Nillable(),

		field.Time("created_at").DefaultFunc(time.Now).Immutable(),
	}
}

// Edges of the Couple.
func (Couple) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("members", User.Type),
		edge.To("calls", Call.Type),
		edge.To("photos", Photo.Type),
	}
}
