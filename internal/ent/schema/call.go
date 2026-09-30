package schema

import (
	"time"

	"github.com/google/uuid"
	"github.com/neko-sc/ent"
	"github.com/neko-sc/ent/dialect/entsql"
	"github.com/neko-sc/ent/schema/edge"
	"github.com/neko-sc/ent/schema/field"
	"github.com/neko-sc/ent/schema/index"

	"cameo/internal/ent/schema_types/call"
)

// Call holds the schema definition for the Call entity.
type Call struct {
	ent.Schema
}

// Fields of the Call.
func (Call) Fields() []ent.Field {
	return []ent.Field{
		field.UUID[uuid.UUID]("id").DefaultFunc(MustUUIDv7).Immutable(),

		field.Int8As[call.Status]("status").DefaultFunc(func() call.Status { return call.StatusRinging }),
		field.Int8As[call.TranscriptStatus]("transcript_status").DefaultFunc(func() call.TranscriptStatus { return call.TranscriptStatusNone }),

		field.String("title").Optional().Nillable(),
		field.String("summary").Optional().Nillable(),
		field.String("recording_key").Optional().Nillable(),
		field.Array[[]uuid.UUID]("favorited_by").Default([]uuid.UUID{}),
		field.Time("started_at").Optional().Nillable(),
		field.Time("ended_at").Optional().Nillable(),

		field.JSON[[]call.Segment]("transcript").Optional(),

		field.UUID[uuid.UUID]("couple_id").Immutable(),
		field.UUID[uuid.UUID]("caller_id").Immutable(),
		field.UUID[uuid.UUID]("callee_id").Immutable(),

		field.Time("created_at").DefaultFunc(time.Now).Immutable(),
		field.Time("updated_at").DefaultFunc(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Call.
func (Call) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("highlights", CallHighlight.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("photos", Photo.Type).Annotations(entsql.OnDelete(entsql.SetNull)),

		edge.From("couple", Couple.Type).
			Ref("calls").
			Field("couple_id").
			Unique().Required().Immutable(),
		edge.From("caller", User.Type).
			Ref("outgoing_calls").
			Field("caller_id").
			Unique().Required().Immutable(),
		edge.From("callee", User.Type).
			Ref("incoming_calls").
			Field("callee_id").
			Unique().Required().Immutable(),
	}
}

// Indexes of the Call.
func (Call) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("couple_id", "created_at", "id").
			Annotations(entsql.DescColumns("created_at", "id")).
			StorageKey("idx_call_list"),

		// At most one ringing (0) or active (1) call per couple.
		index.Fields("couple_id").Unique().
			Annotations(entsql.IndexWhere("status IN (0, 1)")).
			StorageKey("idx_call_live"),

		index.Fields("transcript_status"),
	}
}
