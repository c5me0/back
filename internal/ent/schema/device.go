package schema

import (
	"time"

	"github.com/google/uuid"
	"github.com/neko-sc/ent"
	"github.com/neko-sc/ent/schema/edge"
	"github.com/neko-sc/ent/schema/field"
	"github.com/neko-sc/ent/schema/index"

	"cameo/internal/ent/schema_types/device"
)

// Device holds the schema definition for the Device entity.
type Device struct {
	ent.Schema
}

// Fields of the Device.
func (Device) Fields() []ent.Field {
	return []ent.Field{
		field.UUID[uuid.UUID]("id").DefaultFunc(MustUUIDv7).Immutable(),

		field.Int8As[device.Platform]("platform"),

		field.String("token").NotEmpty().MaxLen(200),

		// Mutable: a token re-registered from another account moves to that account's session.
		field.UUID[uuid.UUID]("user_id"),
		field.UUID[uuid.UUID]("session_id"),

		field.Time("created_at").DefaultFunc(time.Now).Immutable(),
		field.Time("updated_at").DefaultFunc(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Device.
func (Device) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("devices").
			Field("user_id").
			Unique().Required(),
		edge.From("session", Session.Type).
			Ref("devices").
			Field("session_id").
			Unique().Required(),
	}
}

// Indexes of the Device.
func (Device) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("platform", "token").Unique(),
		index.Fields("user_id"),
	}
}
