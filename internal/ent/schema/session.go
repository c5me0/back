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

// Session holds the schema definition for the Session entity.
type Session struct {
	ent.Schema
}

// Fields of the Session.
func (Session) Fields() []ent.Field {
	return []ent.Field{
		field.UUID[uuid.UUID]("id").DefaultFunc(MustUUIDv7).Immutable(),

		field.UUID[uuid.UUID]("user_id").Immutable(),

		field.Time("created_at").DefaultFunc(time.Now).Immutable(),
		field.Time("updated_at").DefaultFunc(time.Now).UpdateDefault(time.Now),
		field.Time("expires_at").DefaultFunc(func() time.Time {
			return time.Now().Add(time.Hour * 24 * 90)
		}),
	}
}

// Edges of the Session.
func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("devices", Device.Type).Annotations(entsql.OnDelete(entsql.Cascade)),

		edge.From("user", User.Type).
			Ref("sessions").
			Field("user_id").
			Unique().Required().Immutable(),
	}
}

// Indexes of the Session.
func (Session) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
	}
}
