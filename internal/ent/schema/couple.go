package schema

import (
	"time"

	"github.com/google/uuid"
	"github.com/neko-sc/ent"
	"github.com/neko-sc/ent/dialect/entsql"
	"github.com/neko-sc/ent/schema/edge"
	"github.com/neko-sc/ent/schema/field"
	"github.com/neko-sc/ent/schema/index"
)

// Couple holds the schema definition for the Couple entity.
type Couple struct {
	ent.Schema
}

// Fields of the Couple.
func (Couple) Fields() []ent.Field {
	return []ent.Field{
		field.UUID[uuid.UUID]("id").DefaultFunc(MustUUIDv7).Immutable(),

		// user_ids keeps the pair, sorted by string form, after disconnect clears users.couple_id.
		// The database default backfills couples created before the column existed.
		field.Array[[]uuid.UUID]("user_ids").Default([]uuid.UUID{}).Immutable().Annotations(entsql.Default("{}")),
		field.Time("disconnected_at").Optional().Nillable(),
		field.Time("restored_at").Optional().Nillable(),
		field.String("restore_transaction_id").Optional().Nillable(),

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

// Indexes of the Couple.
func (Couple) Indexes() []ent.Index {
	return []ent.Index{
		// A restore purchase is consumed by at most one couple.
		index.Fields("restore_transaction_id").Unique().StorageKey("idx_couple_restore_transaction"),
	}
}
