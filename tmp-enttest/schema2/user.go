package schema2

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type User struct{ ent.Schema }

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("super_duper_mega_ultra_hyper_giant_field_name_for_testing"),
		field.String("another_super_duper_mega_ultra_hyper_giant_field_name"),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("super_duper_mega_ultra_hyper_giant_field_name_for_testing", "another_super_duper_mega_ultra_hyper_giant_field_name").Unique(),
	}
}
