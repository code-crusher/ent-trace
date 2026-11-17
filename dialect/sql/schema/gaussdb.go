// Copyright 2019-present Facebook Inc. All rights reserved.
// This source code is licensed under the Apache 2.0 license found
// in the LICENSE file in the root directory of this source tree.

package schema

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"strconv"
	"strings"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/postgres"
)

// GaussDB adapter for Atlas migration engine. GaussDB, and its open-source
// version openGauss, are derived from PostgreSQL and share its SQL syntax
// and semantics. Hence, this adapter reuses the Postgres implementation and
// only adjusts the parts that are specific to GaussDB.
type GaussDB struct {
	Postgres
}

// atParamsQuery is a part of the query executed by the Atlas PostgreSQL
// driver on open. It is used for replacing this query for GaussDB (atOpen),
// as GaussDB does not support all the system variables queried by it.
const atParamsQuery = "current_setting('server_version_num')"

// atVersion is the PostgreSQL version reported to the Atlas driver. GaussDB
// reports its version as PostgreSQL 9.2.4 (90204), which is rejected by the
// Atlas driver (requires >= 10). Version 10 is the minimal version supported
// by the Atlas driver, and it disables features added in later versions
// (e.g. the INCLUDE clause in CREATE INDEX), which are not available in GaussDB.
const atVersion = "100000"

// atVersionQuery is the query used instead of the Atlas one for GaussDB.
const atVersionQuery = "SELECT '" + atVersion + "', NULL, NULL"

// init loads the GaussDB version from the database for later use in the
// migration process. Unlike PostgreSQL, GaussDB reports a version that is
// lower than 10 (9.2.4) for compatibility reasons, and therefore the version
// check done by the Postgres adapter is skipped.
func (d *GaussDB) init(ctx context.Context) error {
	if d.version != "" {
		return nil // already initialized.
	}
	rows := &sql.Rows{}
	if err := d.Query(ctx, "SHOW server_version_num", []any{}, rows); err != nil {
		return fmt.Errorf("querying server version %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return fmt.Errorf("server_version_num variable was not found")
	}
	var version string
	if err := rows.Scan(&version); err != nil {
		return fmt.Errorf("scanning version: %w", err)
	}
	v, err := strconv.Atoi(version)
	if err != nil {
		return fmt.Errorf("malformed version: %s: %w", version, err)
	}
	// The version is formatted as Mmmpp, where M is the major version, and
	// mm and pp are the minor and patch versions. For example, PostgreSQL
	// 13.0.0 is reported as 130000, and openGauss 9.2.4 as 90204.
	d.version = fmt.Sprintf("%d.%d.%d", v/10000, v/100%100, v%100)
	return nil
}

// atOpen opens the Atlas PostgreSQL driver with a connection that reports a
// version supported by the driver, as GaussDB reports its version as 9.2.4.
func (d *GaussDB) atOpen(conn dialect.ExecQuerier) (migrate.Driver, error) {
	return postgres.Open(&gaussdbConn{db{ExecQuerier: conn}})
}

// gaussdbConn wraps the Ent ExecQuerier and replaces the system variables
// query executed by the Atlas PostgreSQL driver on open.
type gaussdbConn struct{ db }

func (c *gaussdbConn) QueryContext(ctx context.Context, query string, args ...any) (*stdsql.Rows, error) {
	if strings.Contains(query, atParamsQuery) {
		query, args = atVersionQuery, nil
	}
	return c.db.QueryContext(ctx, query, args...)
}
