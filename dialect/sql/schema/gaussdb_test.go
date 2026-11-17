// Copyright 2019-present Facebook Inc. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package schema

import (
	"context"
	"regexp"
	"testing"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/schema/field"

	"ariga.io/atlas/sql/schema"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

// TestGaussDB_Migrate tests that a migration can be created for the GaussDB
// dialect, and that the version reported by openGauss (9.2.4) does not fail it.
func TestGaussDB_Migrate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	mock.ExpectQuery(escape("SHOW server_version_num")).
		WillReturnRows(sqlmock.NewRows([]string{"server_version_num"}).AddRow("90204"))
	mock.ExpectQuery(regexp.QuoteMeta(atVersionQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"current_setting", "current_setting", "current_setting"}).
			AddRow(atVersion, nil, nil))
	mock.ExpectQuery("SELECT nspname AS schema_name,.+CURRENT_SCHEMA().+").
		WillReturnRows(sqlmock.NewRows([]string{"schema_name", "comment"}).AddRow("public", "default schema"))
	mock.ExpectQuery("SELECT t3.oid, t1.table_schema,.+").
		WillReturnRows(sqlmock.NewRows([]string{}))
	m, err := NewMigrate(sql.OpenDB(dialect.GaussDB, db), WithDiffHook(func(next Differ) Differ {
		return DiffFunc(func(current, desired *schema.Schema) ([]schema.Change, error) {
			return nil, nil // Noop.
		})
	}))
	require.NoError(t, err)
	require.NoError(t, m.Create(context.Background(), &Table{
		Name:    "users",
		Columns: []*Column{{Name: "id", Type: field.TypeInt, Increment: true}},
		PrimaryKey: []*Column{
			{Name: "id", Type: field.TypeInt, Increment: true},
		},
	}))
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGaussDB_Init tests that the GaussDB adapter accepts the version
// reported by openGauss (9.2.4), which is lower than the minimum version
// required by PostgreSQL.
func TestGaussDB_Init(t *testing.T) {
	for _, version := range []string{"90204", "130000"} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		mock.ExpectQuery(escape("SHOW server_version_num")).
			WillReturnRows(sqlmock.NewRows([]string{"server_version_num"}).AddRow(version))
		d := &GaussDB{Postgres{Driver: sql.OpenDB(dialect.GaussDB, db)}}
		require.NoError(t, d.init(context.Background()))
		require.NotEmpty(t, d.version)
		require.NoError(t, mock.ExpectationsWereMet())
	}
}

// TestGaussDB_AtOpen tests that the Atlas PostgreSQL driver is opened with a
// version supported by it, as GaussDB reports its version as PostgreSQL 9.2.4.
func TestGaussDB_AtOpen(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	mock.ExpectQuery(regexp.QuoteMeta(atVersionQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"current_setting", "current_setting", "current_setting"}).
			AddRow(atVersion, nil, nil))
	d := &GaussDB{Postgres{Driver: sql.OpenDB(dialect.GaussDB, db)}}
	drv, err := d.atOpen(d)
	require.NoError(t, err)
	require.NotNil(t, drv)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestGaussDB_IndexInclude tests that the INCLUDE clause is not emitted for
// GaussDB, as it is not supported by it (added in PostgreSQL 11).
func TestGaussDB_IndexInclude(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	d := &GaussDB{Postgres{Driver: sql.OpenDB(dialect.GaussDB, db), name: dialect.GaussDB, version: "13.0.0"}}
	require.False(t, d.supportsIndexInclude())
	pg := &Postgres{version: "13.0.0"}
	require.True(t, pg.supportsIndexInclude())
}

// TestGaussDB_SchemaType tests that the GaussDB adapter falls back to the
// schema types defined for PostgreSQL, and that it accepts types defined
// for GaussDB.
func TestGaussDB_SchemaType(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	d := &GaussDB{Postgres{Driver: sql.OpenDB(dialect.GaussDB, db), name: dialect.GaussDB}}
	c1 := &Column{Name: "id", Type: field.TypeInt, SchemaType: map[string]string{dialect.Postgres: "bigserial"}}
	require.Equal(t, "bigserial", d.schemaType(c1))
	c2 := &Column{Name: "id", Type: field.TypeInt, SchemaType: map[string]string{
		dialect.Postgres: "bigserial",
		dialect.GaussDB:  "serial",
	}}
	require.Equal(t, "serial", d.schemaType(c2))
	c3 := &Column{Name: "id", Type: field.TypeInt}
	require.Empty(t, d.schemaType(c3))
}

// TestGaussDB_IndexType tests that the GaussDB adapter falls back to the
// index types defined for PostgreSQL.
func TestGaussDB_IndexType(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	d := &GaussDB{Postgres{Driver: sql.OpenDB(dialect.GaussDB, db), name: dialect.GaussDB}}
	idx := &Index{Name: "idx", Annotation: entsql.IndexTypes(map[string]string{dialect.Postgres: "GIN"})}
	t1, ok := d.idxType(idx)
	require.True(t, ok)
	require.Equal(t, "GIN", t1)
	idx = &Index{Name: "idx", Annotation: entsql.IndexTypes(map[string]string{dialect.GaussDB: "GIST"})}
	t2, ok := d.idxType(idx)
	require.True(t, ok)
	require.Equal(t, "GIST", t2)
	_, ok = d.idxType(&Index{Name: "idx"})
	require.False(t, ok)
}
