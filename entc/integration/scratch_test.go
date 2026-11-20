package integration

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/entc/integration/ent"
	"entgo.io/ent/entc/integration/ent/license"
	"entgo.io/ent/entc/integration/ent/migrate"
	"entgo.io/ent/entc/integration/ent/node"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
)

func scratchClient(t *testing.T) *ent.Client {
	ctx := context.Background()
	db, err := sql.Open(dialect.SQLite, "file:scratch?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatal(err)
	}
	client := ent.NewClient(ent.Driver(db))
	if err := client.Schema.Create(ctx, migrate.WithDropIndex(true), migrate.WithDropColumn(true)); err != nil {
		t.Fatal(err)
	}
	return client
}

func TestScratchUpsertBulk(t *testing.T) {
	ctx := context.Background()
	client := scratchClient(t)
	defer client.Close()
	ts := time.Unix(1623279251, 0).UTC()

	// License: update_time has Default + UpdateDefault (Time mixin).
	l1 := client.License.Create().SetCreateTime(ts).SetUpdateTime(ts).SaveX(ctx)
	time.Sleep(time.Second)
	client.License.CreateBulk(client.License.Create().SetID(l1.ID)).
		OnConflictColumns(license.FieldID).
		UpdateNewValues().
		ExecX(ctx)
	l2 := client.License.GetX(ctx, l1.ID)
	t.Log("license update_time:", l1.UpdateTime.Unix(), "->", l2.UpdateTime.Unix())
	if !l2.UpdateTime.After(ts) {
		t.Errorf("license FAIL: update_time was not updated")
	}

	// Node: updated_at has ONLY UpdateDefault.
	n1 := client.Node.Create().SetValue(1).SetUpdatedAt(ts).SaveX(ctx)
	time.Sleep(time.Second)
	client.Node.CreateBulk(client.Node.Create().SetValue(2).SetPrevID(n1.ID)).
		OnConflictColumns(node.FieldID).
		UpdateNewValues().
		ExecX(ctx)
	n2 := client.Node.GetX(ctx, n1.ID)
	t.Log("node updated_at:", n1.UpdatedAt.Unix(), "->", n2.UpdatedAt.Unix())
	if !n2.UpdatedAt.After(ts) {
		t.Errorf("node FAIL: updated_at was not updated")
	}

	// Manual set of a UpdateDefault-only field on the builder.
	client.Node.CreateBulk(client.Node.Create().SetValue(3).SetPrevID(n1.ID).SetUpdatedAt(time.Now())).
		OnConflictColumns(node.FieldID).
		UpdateNewValues().
		ExecX(ctx)
	n3 := client.Node.GetX(ctx, n1.ID)
	t.Log("node(manual) updated_at:", n1.UpdatedAt.Unix(), "->", n3.UpdatedAt.Unix())
}
