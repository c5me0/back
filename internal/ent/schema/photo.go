package schema

import (
	"time"

	"github.com/google/uuid"
	"github.com/neko-sc/ent"
	"github.com/neko-sc/ent/dialect/entsql"
	"github.com/neko-sc/ent/schema/edge"
	"github.com/neko-sc/ent/schema/field"
	"github.com/neko-sc/ent/schema/index"

	"cameo/internal/ent/schema_types/photo"
)

// Photo holds the schema definition for the Photo entity.
type Photo struct {
	ent.Schema
}

// Fields of the Photo.
func (Photo) Fields() []ent.Field {
	return []ent.Field{
		field.UUID[uuid.UUID]("id").DefaultFunc(MustUUIDv7).Immutable(),

		field.Int8As[photo.Status]("status").DefaultFunc(func() photo.Status { return photo.StatusPending }),

		field.String("content_type").NotEmpty(),
		field.String("object_key").NotEmpty().Immutable(),
		field.String("thumbnail_key").NotEmpty().Immutable(),
		field.Int64("size_bytes"),
		field.Int("width").Optional().Nillable(),
		field.Int("height").Optional().Nillable(),
		field.Time("taken_at").Optional().Nillable(),
		field.Array[[]uuid.UUID]("favorited_by").Default([]uuid.UUID{}),

		field.UUID[uuid.UUID]("couple_id"),
		field.UUID[uuid.UUID]("uploader_id").Immutable(),
		field.UUID[uuid.UUID]("call_id").Optional().Nillable(),

		field.Time("created_at").DefaultFunc(time.Now).Immutable(),
		field.Time("updated_at").DefaultFunc(time.Now).UpdateDefault(time.Now),
	}
}

// Edges of the Photo.
func (Photo) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("couple", Couple.Type).
			Ref("photos").
			Field("couple_id").
			Unique().Required(),
		edge.From("uploader", User.Type).
			Ref("photos").
			Field("uploader_id").
			Unique().Required().Immutable(),
		edge.From("call", Call.Type).
			Ref("photos").
			Field("call_id").
			Unique(),
	}
}

// Indexes of the Photo.
func (Photo) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("couple_id", "created_at", "id").
			Annotations(entsql.DescColumns("created_at", "id")).
			StorageKey("idx_photo_list"),
		index.Fields("call_id"),
	}
}
