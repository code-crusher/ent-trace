// Copyright 2019-present Facebook Inc. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package sql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	"entgo.io/ent/dialect"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithVars(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	drv := OpenDB(dialect.Postgres, db)
	mock.ExpectExec("SET foo = 'bar'").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec("RESET foo").WillReturnResult(sqlmock.NewResult(0, 0))
	rows := &Rows{}
	err = drv.Query(
		WithVar(context.Background(), "foo", "bar"),
		"SELECT 1",
		[]any{},
		rows,
	)
	require.NoError(t, err)
	require.NoError(t, rows.Close(), "rows should be closed to release the connection")
	require.NoError(t, mock.ExpectationsWereMet())

	mock.ExpectExec("SET foo = 'bar'").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SET foo = 'baz'").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec("RESET foo").WillReturnResult(sqlmock.NewResult(0, 0))
	err = drv.Query(
		WithVar(WithVar(context.Background(), "foo", "bar"), "foo", "baz"),
		"SELECT 1",
		[]any{},
		rows,
	)
	require.NoError(t, err)
	require.NoError(t, rows.Close(), "rows should be closed to release the connection")
	require.NoError(t, mock.ExpectationsWereMet())

	mock.ExpectBegin()
	mock.ExpectExec("SET foo = 'bar'").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT 1").WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectCommit()
	tx, err := drv.Tx(context.Background())
	require.NoError(t, err)
	err = tx.Query(
		WithVar(context.Background(), "foo", "bar"),
		"SELECT 1",
		[]any{},
		rows,
	)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
	// Rows should not be closed to release the session,
	// as a transaction is always scoped to a single connection.

	mock.ExpectExec("SET foo = 'qux'").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO users DEFAULT VALUES").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RESET foo").WillReturnResult(sqlmock.NewResult(0, 0))
	err = drv.Exec(
		WithVar(context.Background(), "foo", "qux"),
		"INSERT INTO users DEFAULT VALUES",
		[]any{},
		nil,
	)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	// No rows are returned, so no need to close them.

	mock.ExpectExec("SET foo = 'foo'").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO users DEFAULT VALUES").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("RESET foo").WillReturnResult(sqlmock.NewResult(0, 0))
	err = drv.Exec(
		WithVar(context.Background(), "foo", "foo"),
		"INSERT INTO users DEFAULT VALUES",
		[]any{},
		nil,
	)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	// No rows are returned, so no need to close them.
}

func TestRefresh(t *testing.T) {
	d := &sourceDriver{sources: make(map[string]int)}
	sql.Register("refresh", d)
	drv, err := Open("refresh", "source-1")
	require.NoError(t, err)
	require.Equal(t, "refresh", drv.Dialect())

	// Queries are executed on the original source.
	require.NoError(t, drv.Query(context.Background(), "SELECT 1", []any{}, &Rows{}))
	require.NoError(t, drv.Exec(context.Background(), "SELECT 1", []any{}, nil))
	require.Positive(t, d.opened("source-1"))
	require.Zero(t, d.opened("source-2"))

	// Refreshing the driver with a new source replaces the connection
	// pool, and queries are executed on the new source from now on.
	old := drv.DB()
	require.NoError(t, drv.Refresh(context.Background(), "source-2"))
	require.NotSame(t, old, drv.DB())
	require.Error(t, old.Ping())
	require.Equal(t, 1, d.opened("source-2"))
	require.NoError(t, drv.Query(context.Background(), "SELECT 2", []any{}, &Rows{}))
	require.NoError(t, drv.Exec(context.Background(), "SELECT 2", []any{}, nil))

	// A closed driver cannot be refreshed.
	require.NoError(t, drv.Close())
	require.EqualError(t, drv.Refresh(context.Background(), "source-3"), "dialect/sql: refresh a closed driver")
}

func TestRefreshInvalidSource(t *testing.T) {
	d := &sourceDriver{sources: make(map[string]int), err: errors.New("bad credentials")}
	sql.Register("refresh_invalid", d)
	drv, err := Open("refresh_invalid", "source-1")
	require.NoError(t, err)
	db := drv.DB()
	require.ErrorIs(t, drv.Refresh(context.Background(), "source-2"), d.err)
	// A failed refresh keeps the previous connection pool in use.
	require.Same(t, db, drv.DB())
	require.NoError(t, drv.Close())
}

func TestRefreshUnsupported(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	drv := OpenDB(dialect.MySQL, db)
	require.EqualError(t, drv.Refresh(context.Background(), "source"), "dialect/sql: refresh is supported only for drivers created with Open")
	require.Same(t, db, drv.DB())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRefreshConcurrent(t *testing.T) {
	sql.Register("refresh_concurrent", &sourceDriver{sources: make(map[string]int)})
	drv, err := Open("refresh_concurrent", "source-1")
	require.NoError(t, err)
	var wg sync.WaitGroup
	wg.Add(2)
	// Queries and refreshes are executed concurrently to ensure
	// the connection pool is replaced atomically and safely.
	go func() {
		defer wg.Done()
		for i := range 100 {
			rows := &Rows{}
			assert.NoError(t, drv.Query(context.Background(), fmt.Sprintf("SELECT %d", i), []any{}, rows))
		}
	}()
	go func() {
		defer wg.Done()
		for i := range 100 {
			assert.NoError(t, drv.Refresh(context.Background(), fmt.Sprintf("source-%d", i)))
		}
	}()
	wg.Wait()
	require.NoError(t, drv.Close())
}

// sourceDriver is a minimal database/sql driver that
// records the sources it was opened with.
type sourceDriver struct {
	mu      sync.Mutex
	sources map[string]int
	err     error
}

// Open implements the driver.Driver interface.
func (d *sourceDriver) Open(source string) (driver.Conn, error) {
	if d.err != nil {
		return nil, d.err
	}
	d.mu.Lock()
	d.sources[source]++
	d.mu.Unlock()
	return &sourceConn{}, nil
}

// opened reports the number of connections opened for the given source.
func (d *sourceDriver) opened(source string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.sources[source]
}

// sourceConn is a no-op driver.Conn.
type sourceConn struct{}

func (*sourceConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (*sourceConn) Close() error                        { return nil }
func (*sourceConn) Begin() (driver.Tx, error)           { return nil, errors.New("not implemented") }

// QueryContext implements the driver.QueryerContext interface.
func (*sourceConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &sourceRows{}, nil
}

// ExecContext implements the driver.ExecerContext interface.
func (*sourceConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.ResultNoRows, nil
}

// Ping implements the driver.Pinger interface.
func (*sourceConn) Ping(context.Context) error { return nil }

// sourceRows is a no-op driver.Rows.
type sourceRows struct{}

func (*sourceRows) Columns() []string         { return []string{"c"} }
func (*sourceRows) Close() error              { return nil }
func (*sourceRows) Next([]driver.Value) error { return io.EOF }
