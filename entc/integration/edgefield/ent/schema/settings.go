// Copyright 2019-present Facebook Inc. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Settings holds the schema definition for the Settings entity.
type Settings struct {
	ent.Schema
}

// Fields of the Settings.
func (Settings) Fields() []ent.Field {
	return []ent.Field{
		// The "id" field is referenced by the "account" edge as the
		// foreign-key, and therefore, it is also the primary-key of
		// the table (Issue 4374).
		field.UUID("id", uuid.UUID{}).
			Immutable(),
		field.String("theme").
			Default("light"),
	}
}

// Edges of the Settings.
func (Settings) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("account", Account.Type).
			Ref("settings").
			Field("id").
			Unique().
			Required().
			Immutable(),
	}
}
