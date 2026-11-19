// Copyright 2019-present Facebook Inc. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package main

import (
	"context"
	"fmt"
	"log"

	"entgo.io/ent/dialect/sql"

	"entgo.io/ent/examples/o2mrecur/ent"
	"entgo.io/ent/examples/o2mrecur/ent/node"

	_ "github.com/mattn/go-sqlite3"
)

func Example_o2mRecur() {
	client, err := ent.Open("sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	if err != nil {
		log.Fatalf("failed opening connection to sqlite: %v", err)
	}
	defer client.Close()
	ctx := context.Background()
	// Run the auto migration tool.
	if err := client.Schema.Create(ctx); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}
	if err := Do(ctx, client); err != nil {
		log.Fatal(err)
	}
	// Output:
	// Tree leafs [1 3 5]
	// [1 3 5]
	// Node(id=1, value=2, parent_id=0)
	// WithDescendants: [Node(id=1, value=2, parent_id=0) Node(id=2, value=1, parent_id=1) Node(id=3, value=4, parent_id=1) Node(id=4, value=3, parent_id=3) Node(id=5, value=5, parent_id=3)]
}

func Do(ctx context.Context, client *ent.Client) error {
	root, err := client.Node.
		Create().
		SetValue(2).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("creating the root: %w", err)
	}

	// Add additional nodes to the tree:
	//
	//       2
	//     /   \
	//    1     4
	//        /   \
	//       3     5
	//

	// Unlike `Save`, `SaveX` panics if an error occurs.
	n1 := client.Node.
		Create().
		SetValue(1).
		SetParent(root).
		SaveX(ctx)
	n4 := client.Node.
		Create().
		SetValue(4).
		SetParent(root).
		SaveX(ctx)
	n3 := client.Node.
		Create().
		SetValue(3).
		SetParent(n4).
		SaveX(ctx)
	n5 := client.Node.
		Create().
		SetValue(5).
		SetParent(n4).
		SaveX(ctx)

	fmt.Println("Tree leafs", []int{n1.Value, n3.Value, n5.Value})
	// Output: Tree leafs [1 3 5]

	// Get all leafs (nodes without children).
	// Unlike `Int`, `IntX` panics if an error occurs.
	ints := client.Node.
		Query().                             // All nodes.
		Where(node.Not(node.HasChildren())). // Only leafs.
		Order(ent.Asc(node.FieldValue)).     // Order by their `value` field.
		GroupBy(node.FieldValue).            // Extract only the `value` field.
		IntsX(ctx)
	fmt.Println(ints)
	// Output: [1 3 5]

	// Get orphan nodes (nodes without parent).
	// Unlike `Only`, `OnlyX` panics if an error occurs.
	orphan := client.Node.
		Query().
		Where(node.Not(node.HasParent())).
		OnlyX(ctx)
	fmt.Println(orphan)
	// Output: Node(id=1, value=2, parent_id=0)

	// Get the root node with all its descendants (until the leaf nodes).
	descendants, err := WithDescendants(ctx, client, root)
	if err != nil {
		return fmt.Errorf("loading descendants: %w", err)
	}
	fmt.Println("WithDescendants:", descendants)
	return nil
}

// WithDescendants returns the given node with all its descendants (until the
// leaf nodes) as a flat list. Note that, eager-loading (e.g. WithChildren)
// loads a static number of levels, and therefore, cannot be used for loading
// a tree of unknown depth. In such cases, a recursive CTE can be used as follows.
//
// This function assumes the schema uses an edge field named "parent_id" to
// store the parent identifier (i.e. Field("parent_id")).
func WithDescendants(ctx context.Context, client *ent.Client, n *ent.Node) ([]*ent.Node, error) {
	return client.
		Node.
		Query().
		Where(func(s *sql.Selector) {
			t1 := sql.Table(node.Table)
			with := sql.WithRecursive("tree", node.FieldID, node.FieldParentID)
			with.As(
				// The initial `SELECT` statement is executed once at the start,
				// and produces the initial row for the recursion.
				sql.Select(t1.Columns(node.FieldID, node.FieldParentID)...).
					From(t1).
					Where(sql.EQ(t1.C(node.FieldID), n.ID)).
					// Merge the `SELECT` statement above with the following:
					UnionAll(
						// A `SELECT` statement that produces additional rows and recurses
						// by referring to the CTE name ("tree"), and ends when there are
						// no more new rows.
						sql.Select(t1.Columns(node.FieldID, node.FieldParentID)...).
							From(t1).
							Join(with).
							On(t1.C(node.FieldParentID), with.C(node.FieldID)),
					),
			)
			// Join the root `SELECT` query with the CTE result (`WITH` clause).
			s.Prefix(with).Join(with).On(s.C(node.FieldID), with.C(node.FieldID))
		}).
		Order(ent.Asc(node.FieldID)).
		All(ctx)
}
