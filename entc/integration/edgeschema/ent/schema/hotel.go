// Copyright 2019-present Facebook Inc. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Hotel holds the schema definition for the Hotel entity.
type Hotel struct {
	ent.Schema
}

// Fields of the Hotel.
func (Hotel) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
	}
}

// Edges of the Hotel.
func (Hotel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("guests", User.Type).
			Ref("hotel_bookings").
			Through("bookings", Booking.Type),
	}
}
