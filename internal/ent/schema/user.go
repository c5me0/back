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

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.UUID[uuid.UUID]("id").DefaultFunc(MustUUIDv7).Immutable(),

		field.String("phone").NotEmpty().Immutable(),
		field.String("display_name").Optional().Nillable(),
		field.String("pairing_code").NotEmpty(),
		field.Bool("call_alert").Default(true),
		field.Bool("highlight_alert").Default(true),
		field.Time("premium_until").Optional().Nillable(),
		field.Array[[]string]("restore_transaction_ids").Default([]string{}).Annotations(entsql.Default("{}")),
		field.Time("purchases_synced_at").Optional().Nillable(),

		field.UUID[uuid.UUID]("couple_id").Optional().Nillable(),

		field.Time("created_at").DefaultFunc(time.Now).Immutable(),
		field.Time("updated_at").DefaultFunc(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("sessions", Session.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("devices", Device.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("outgoing_calls", Call.Type),
		edge.To("incoming_calls", Call.Type),
		edge.To("call_highlights", CallHighlight.Type),
		edge.To("photos", Photo.Type),

		edge.From("couple", Couple.Type).
			Ref("members").
			Field("couple_id").
			Unique(),
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("phone").Unique(),
		index.Fields("pairing_code").Unique(),
	}
}
