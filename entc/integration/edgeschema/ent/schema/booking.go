// Copyright 2019-present Facebook Inc. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Booking holds the schema definition for the Booking edge schema.
// It allows a person to book the same hotel more than once (e.g. at
// different points in time), so its index on the (user_id, hotel_id)
// columns is not unique. Defining the index explicitly overrides the
// unique index that Ent generates by default for edge-schemas.
type Booking struct {
	ent.Schema
}

// Fields of the Booking.
func (Booking) Fields() []ent.Field {
	return []ent.Field{
		field.Time("booked_at").
			Default(time.Now),
		field.Int("user_id"),
		field.Int("hotel_id"),
	}
}

// Edges of the Booking.
func (Booking) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).
			Unique().
			Required().
			Field("user_id"),
		edge.To("hotel", Hotel.Type).
			Unique().
			Required().
			Field("hotel_id"),
	}
}

// Indexes of the Booking.
func (Booking) Indexes() []ent.Index {
	return []ent.Index{
		// By default, Ent generates a unique index on the edge-fields of
		// edge-schemas with an ID field. Defining an index on the same
		// columns (in either order) overrides this behavior. In this case,
		// we allow the same user to book the same hotel multiple times.
		index.Fields("user_id", "hotel_id"),
	}
}
