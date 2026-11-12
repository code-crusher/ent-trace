package integration

import (
	"context"
	"testing"

	"entgo.io/ent"
	ent2 "entgo.io/ent/entc/integration/ent"
	"entgo.io/ent/entc/integration/ent/enttest"
	"entgo.io/ent/entc/integration/ent/pet"

	"github.com/stretchr/testify/require"
)

func TestReproEagerLoading(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent_repro2?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	a8m := client.User.Create().SetName("a8m").SetAge(30).SaveX(ctx)
	nati := client.User.Create().SetName("nati").SetAge(30).SaveX(ctx)
	client.Pet.Create().SetName("xabi").SetOwner(a8m).SaveX(ctx)
	client.Pet.Create().SetName("luna").SetOwner(nati).SaveX(ctx)

	client.Intercept(ent.TraverseFunc(func(ctx context.Context, q ent.Query) error {
		switch q := q.(type) {
		case *ent2.PetQuery:
			q.Where(pet.Name("xabi"))
		}
		return nil
	}))

	users := client.User.Query().WithPets().AllX(ctx)
	total := 0
	for _, u := range users {
		total += len(u.Edges.Pets)
	}
	t.Logf("eager-loaded pets total: %d", total)
	require.Equal(t, 1, total)
}
