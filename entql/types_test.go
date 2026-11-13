// Copyright 2019-present Facebook Inc. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package entql

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFielder(t *testing.T) {
	tests := []struct {
		input    Fielder
		expected string
	}{
		{
			input:    StringEQ("value"),
			expected: `field == "value"`,
		},
		{
			input: StringOr(
				StringEQ("a"),
				StringEQ("b"),
				StringEQ("c"),
			),
			expected: `(field == "a" || field == "b" || field == "c")`,
		},
		{
			input: StringAnd(
				StringEQ("a"),
				StringNot(
					StringOr(
						StringEQ("b"),
						StringGT("c"),
						StringNEQ("d"),
					),
				),
			),
			expected: `field == "a" && !((field == "b" || field > "c" || field != "d"))`,
		},
		{
			input:    IntGT(1),
			expected: `field > 1`,
		},
		{
			input:    IntGTE(1),
			expected: `field >= 1`,
		},
		{
			input:    IntLT(1),
			expected: `field < 1`,
		},
		{
			input:    IntLTE(1),
			expected: `field <= 1`,
		},
		{
			input:    IntGT(1),
			expected: `field > 1`,
		},
		{
			input:    IntNot(IntGTE(1)),
			expected: `!(field >= 1)`,
		},
		{
			input: BoolNot(
				BoolOr(
					BoolEQ(true),
					BoolEQ(false),
				),
			),
			expected: `!(field == true || field == false)`,
		},
		{
			input:    IntIn(1, 2, 3),
			expected: `field in [1,2,3]`,
		},
		{
			input:    IntNotIn(1, 2, 3),
			expected: `field not in [1,2,3]`,
		},
		{
			input:    Uint64In(1, 2),
			expected: `field in [1,2]`,
		},
		{
			input:    Float64NotIn(1.5, 2.5),
			expected: `field not in [1.5,2.5]`,
		},
		{
			input:    TimeIn(time.Unix(0, 0), time.Unix(1, 0)),
			expected: `field in ["1970-01-01T00:00:00Z","1970-01-01T00:00:01Z"]`,
		},
		{
			input:    StringIn("a", "b"),
			expected: `field in ["a","b"]`,
		},
		{
			input:    StringNotIn("a", "b"),
			expected: `field not in ["a","b"]`,
		},
		{
			input:    StringContains("a"),
			expected: `contains(field, "a")`,
		},
		{
			input:    StringContainsFold("a"),
			expected: `contains_fold(field, "a")`,
		},
		{
			input:    StringEqualFold("a"),
			expected: `equal_fold(field, "a")`,
		},
		{
			input:    StringHasPrefix("a"),
			expected: `has_prefix(field, "a")`,
		},
		{
			input:    StringHasSuffix("a"),
			expected: `has_suffix(field, "a")`,
		},
	}
	for i := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			p := tests[i].input.Field("field")
			assert.Equal(t, tests[i].expected, p.String())
		})
	}
}
