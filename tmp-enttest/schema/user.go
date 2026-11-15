package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type User struct{ ent.Schema }

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.String("email"),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "email").Unique(),
	}
}
