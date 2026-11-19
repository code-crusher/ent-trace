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
	"strconv"
	"strings"
	"sync"

	"entgo.io/ent/dialect"
)

// Driver is a dialect.Driver implementation for SQL based databases.
type Driver struct {
	Conn
	dialect string
	// driverName is the name of the database driver that was used
	// for opening the connection. It is set only by Open, and used
	// by Refresh for re-opening the connection with a new source.
	driverName string
	// mu protects the Conn and closed fields from concurrent access
	// by queries and Refresh or Close calls.
	mu     sync.RWMutex
	closed bool
	// refreshMu serializes Refresh calls, as it is executed without
	// holding mu to avoid blocking queries on network operations.
	refreshMu sync.Mutex
}

// NewDriver creates a new Driver with the given Conn and dialect.
func NewDriver(dialect string, c Conn) *Driver {
	return &Driver{dialect: dialect, Conn: c}
}

// Open wraps the database/sql.Open method and returns a dialect.Driver that implements the an ent/dialect.Driver interface.
func Open(driverName, source string) (*Driver, error) {
	db, err := sql.Open(driverName, source)
	if err != nil {
		return nil, err
	}
	d := NewDriver(driverName, Conn{db, driverName})
	d.driverName = driverName
	return d, nil
}

// OpenDB wraps the given database/sql.DB method with a Driver.
func OpenDB(dialect string, db *sql.DB) *Driver {
	return NewDriver(dialect, Conn{db, dialect})
}

// DB returns the underlying *sql.DB instance.
func (d *Driver) DB() *sql.DB {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db()
}

// db returns the underlying *sql.DB instance without holding the lock.
func (d *Driver) db() *sql.DB {
	return d.ExecQuerier.(*sql.DB)
}

// Dialect implements the dialect.Dialect method.
func (d *Driver) Dialect() string {
	// If the underlying driver is wrapped with a telemetry driver.
	for _, name := range []string{dialect.MySQL, dialect.SQLite, dialect.Postgres} {
		if strings.HasPrefix(d.dialect, name) {
			return name
		}
	}
	return d.dialect
}

// Exec implements the dialect.Exec method.
func (d *Driver) Exec(ctx context.Context, query string, args, v any) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Conn.Exec(ctx, query, args, v)
}

// Query implements the dialect.Query method.
func (d *Driver) Query(ctx context.Context, query string, args, v any) error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Conn.Query(ctx, query, args, v)
}

// Tx starts and returns a transaction.
func (d *Driver) Tx(ctx context.Context) (dialect.Tx, error) {
	return d.BeginTx(ctx, nil)
}

// BeginTx starts a transaction with options.
func (d *Driver) BeginTx(ctx context.Context, opts *TxOptions) (dialect.Tx, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	tx, err := d.db().BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{
		Conn: Conn{tx, d.dialect},
		Tx:   tx,
	}, nil
}

// Close closes the underlying connection.
func (d *Driver) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	return d.db().Close()
}

// Refresh replaces the underlying connection pool with a new one that is
// created from the given data source name. It allows refreshing the data
// source of a running application, such as the address, username or
// password of the database, without restarting it. For example:
//
//	drv, err := sql.Open("mysql", "<old-dsn>")
//	if err != nil {
//		return err
//	}
//	client := ent.NewClient(ent.Driver(drv))
//	// When the data source is changed (e.g. rotated credentials):
//	if err := drv.Refresh(ctx, "<new-dsn>"); err != nil {
//		return err
//	}
//
// The new data source is verified by pinging the database before the
// existing pool is replaced. Hence, if the new data source is invalid,
// the driver keeps using the previous one. After a successful refresh,
// the previous pool is closed. Connections that are still in use, such
// as those held by running transactions, are closed once they are done.
//
// Note, Refresh is supported only for drivers that were created with Open.
func (d *Driver) Refresh(ctx context.Context, source string) error {
	d.refreshMu.Lock()
	defer d.refreshMu.Unlock()
	// driverName is set once by Open and is immutable.
	if d.driverName == "" {
		return errors.New("dialect/sql: refresh is supported only for drivers created with Open")
	}
	if d.isClosed() {
		return errors.New("dialect/sql: refresh a closed driver")
	}
	// The new pool is opened and verified before acquiring the write
	// lock, to avoid blocking in-flight queries on network operations.
	db, err := sql.Open(d.driverName, source)
	if err != nil {
		return fmt.Errorf("dialect/sql: open refreshed connection: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("dialect/sql: ping refreshed connection: %w", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	// The driver may have been closed while the new pool was verified.
	if d.closed {
		_ = db.Close()
		return errors.New("dialect/sql: refresh a closed driver")
	}
	old := d.db()
	d.ExecQuerier = db
	if err := old.Close(); err != nil {
		return fmt.Errorf("dialect/sql: close previous connection: %w", err)
	}
	return nil
}

// isClosed reports whether the driver was closed.
func (d *Driver) isClosed() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.closed
}

// Tx implements dialect.Tx interface.
type Tx struct {
	Conn
	driver.Tx
}

// ctyVarsKey is the key used for attaching and reading the context variables.
type ctxVarsKey struct{}

// sessionVars holds sessions/transactions variables to set before every statement.
type sessionVars struct {
	vars []struct{ k, v string }
}

// WithVar returns a new context that holds the session variable to be executed before every query.
func WithVar(ctx context.Context, name, value string) context.Context {
	sv, _ := ctx.Value(ctxVarsKey{}).(sessionVars)
	sv.vars = append(sv.vars, struct {
		k, v string
	}{
		k: name,
		v: value,
	})
	return context.WithValue(ctx, ctxVarsKey{}, sv)
}

// VarFromContext returns the session variable value from the context.
func VarFromContext(ctx context.Context, name string) (string, bool) {
	sv, _ := ctx.Value(ctxVarsKey{}).(sessionVars)
	for _, s := range sv.vars {
		if s.k == name {
			return s.v, true
		}
	}
	return "", false
}

// WithIntVar calls WithVar with the string representation of the value.
func WithIntVar(ctx context.Context, name string, value int) context.Context {
	return WithVar(ctx, name, strconv.Itoa(value))
}

// ExecQuerier wraps the standard Exec and Query methods.
type ExecQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Conn implements dialect.ExecQuerier given ExecQuerier.
type Conn struct {
	ExecQuerier
	dialect string
}

// Exec implements the dialect.Exec method.
func (c Conn) Exec(ctx context.Context, query string, args, v any) (rerr error) {
	argv, ok := args.([]any)
	if !ok {
		return fmt.Errorf("dialect/sql: invalid type %T. expect []any for args", v)
	}
	ex, cf, err := c.maySetVars(ctx)
	if err != nil {
		return err
	}
	if cf != nil {
		defer func() { rerr = errors.Join(rerr, cf()) }()
	}
	switch v := v.(type) {
	case nil:
		if _, err := ex.ExecContext(ctx, query, argv...); err != nil {
			return err
		}
	case *sql.Result:
		res, err := ex.ExecContext(ctx, query, argv...)
		if err != nil {
			return err
		}
		*v = res
	default:
		return fmt.Errorf("dialect/sql: invalid type %T. expect *sql.Result", v)
	}
	return nil
}

// Query implements the dialect.Query method.
func (c Conn) Query(ctx context.Context, query string, args, v any) error {
	vr, ok := v.(*Rows)
	if !ok {
		return fmt.Errorf("dialect/sql: invalid type %T. expect *sql.Rows", v)
	}
	argv, ok := args.([]any)
	if !ok {
		return fmt.Errorf("dialect/sql: invalid type %T. expect []any for args", args)
	}
	ex, cf, err := c.maySetVars(ctx)
	if err != nil {
		return err
	}
	rows, err := ex.QueryContext(ctx, query, argv...)
	if err != nil {
		if cf != nil {
			err = errors.Join(err, cf())
		}
		return err
	}
	*vr = Rows{rows}
	if cf != nil {
		vr.ColumnScanner = rowsWithCloser{rows, cf}
	}
	return nil
}

// maySetVars sets the session variables before executing a query.
func (c Conn) maySetVars(ctx context.Context) (ExecQuerier, func() error, error) {
	sv, _ := ctx.Value(ctxVarsKey{}).(sessionVars)
	if len(sv.vars) == 0 {
		return c, nil, nil
	}
	var (
		ex    ExecQuerier  // Underlying ExecQuerier.
		cf    func() error // Close function.
		reset []string     // Reset variables.
		seen  = make(map[string]struct{}, len(sv.vars))
	)
	switch e := c.ExecQuerier.(type) {
	case *sql.Tx:
		ex = e
	case *sql.DB:
		conn, err := e.Conn(ctx)
		if err != nil {
			return nil, nil, err
		}
		ex, cf = conn, conn.Close
	}
	for _, s := range sv.vars {
		if _, ok := seen[s.k]; !ok {
			switch c.dialect {
			case dialect.Postgres:
				reset = append(reset, fmt.Sprintf("RESET %s", s.k))
			case dialect.MySQL:
				reset = append(reset, fmt.Sprintf("SET %s = NULL", s.k))
			}
			seen[s.k] = struct{}{}
		}
		if _, err := ex.ExecContext(ctx, fmt.Sprintf("SET %s = '%s'", s.k, s.v)); err != nil {
			if cf != nil {
				err = errors.Join(err, cf())
			}
			return nil, nil, err
		}
	}
	// If there are variables to reset, and we need to return the
	// connection to the pool, we need to clean up the variables.
	if cls := cf; cf != nil && len(reset) > 0 {
		cf = func() error {
			for _, q := range reset {
				if _, err := ex.ExecContext(ctx, q); err != nil {
					return errors.Join(err, cls())
				}
			}
			return cls()
		}
	}
	return ex, cf, nil
}

var _ dialect.Driver = (*Driver)(nil)

type (
	// Rows wraps the sql.Rows to avoid locks copy.
	Rows struct{ ColumnScanner }
	// Result is an alias to sql.Result.
	Result = sql.Result
	// NullBool is an alias to sql.NullBool.
	NullBool = sql.NullBool
	// NullInt64 is an alias to sql.NullInt64.
	NullInt64 = sql.NullInt64
	// NullString is an alias to sql.NullString.
	NullString = sql.NullString
	// NullFloat64 is an alias to sql.NullFloat64.
	NullFloat64 = sql.NullFloat64
	// NullTime represents a time.Time that may be null.
	NullTime = sql.NullTime
	// TxOptions holds the transaction options to be used in DB.BeginTx.
	TxOptions = sql.TxOptions
)

// NullScanner implements the sql.Scanner interface such that it
// can be used as a scan destination, similar to the types above.
type NullScanner struct {
	S     sql.Scanner
	Valid bool // Valid is true if the Scan value is not NULL.
}

// Scan implements the Scanner interface.
func (n *NullScanner) Scan(value any) error {
	n.Valid = value != nil
	if n.Valid {
		return n.S.Scan(value)
	}
	return nil
}

// ColumnScanner is the interface that wraps the standard
// sql.Rows methods used for scanning database rows.
type ColumnScanner interface {
	Close() error
	ColumnTypes() ([]*sql.ColumnType, error)
	Columns() ([]string, error)
	Err() error
	Next() bool
	NextResultSet() bool
	Scan(dest ...any) error
}

// rowsWithCloser wraps the ColumnScanner interface with a custom Close hook.
type rowsWithCloser struct {
	ColumnScanner
	closer func() error
}

// Close closes the underlying ColumnScanner and calls the custom closer.
func (r rowsWithCloser) Close() error {
	err := r.ColumnScanner.Close()
	return errors.Join(err, r.closer())
}
