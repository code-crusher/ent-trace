package schema4

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type User struct{ ent.Schema }

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("super_duper_mega_ultra_hyper_giant_field_name_for_testing_purposes").Unique(),
		field.String("another_super_duper_mega_ultra_hyper_giant_field_name_longer"),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("pets_with_really_long_edge_name_for_testing_purposes_even_longer", Pet.Type),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("super_duper_mega_ultra_hyper_giant_field_name_for_testing_purposes", "another_super_duper_mega_ultra_hyper_giant_field_name_longer").Unique(),
	}
}

type Pet struct{ ent.Schema }

func (Pet) Fields() []ent.Field { return []ent.Field{field.String("name")} }
