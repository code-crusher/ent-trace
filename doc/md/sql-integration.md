---
id: sql-integration
title: sql.DB Integration
---

The following examples show how to pass a custom `sql.DB` object to `ent.Client`.

## Configure `sql.DB`

First option:

```go
package main

import (
    "time"

    "<your_project>/ent"
    "entgo.io/ent/dialect/sql"
)

func Open() (*ent.Client, error) {
    drv, err := sql.Open("mysql", "<mysql-dsn>")
    if err != nil {
    	return nil, err
    }
    // Get the underlying sql.DB object of the driver.
    db := drv.DB()
    db.SetMaxIdleConns(10)
    db.SetMaxOpenConns(100)
    db.SetConnMaxLifetime(time.Hour)
    return ent.NewClient(ent.Driver(drv)), nil
}
```

Second option:

```go
package main

import (
    "database/sql"
    "time"

    "<your_project>/ent"
    entsql "entgo.io/ent/dialect/sql"
)

func Open() (*ent.Client, error) {
    db, err := sql.Open("mysql", "<mysql-dsn>")
    if err != nil {
    	return nil, err
    }
    db.SetMaxIdleConns(10)
    db.SetMaxOpenConns(100)
    db.SetConnMaxLifetime(time.Hour)
    // Create an ent.Driver from `db`.
    drv := entsql.OpenDB("mysql", db)
    return ent.NewClient(ent.Driver(drv)), nil
}
```

## Refresh the Data Source at Runtime

The `Driver` returned by `sql.Open` supports replacing its data source (i.e. the
address, username or password of the database) at runtime using the `Refresh`
method. This allows rotating database credentials, or switching to a different
database, without restarting the application:

```go
package main

import (
    "context"
    "log"

    "<your_project>/ent"
    "entgo.io/ent/dialect/sql"
)

func main() {
    drv, err := sql.Open("mysql", "<old-dsn>")
    if err != nil {
        log.Fatal(err)
    }
    client := ent.NewClient(ent.Driver(drv))
    ctx := context.Background()

    // When the data source is changed (e.g. rotated credentials):
    if err := drv.Refresh(ctx, "<new-dsn>"); err != nil {
        log.Fatal(err)
    }
    // The client keeps using the same driver, but new queries are
    // executed against the new data source.
    if err := client.User.Create().SetName("a8m").Exec(ctx); err != nil {
        log.Fatal(err)
    }
}
```

The new data source is verified by pinging the database before replacing the
connection pool, so a failed refresh (e.g. wrong credentials) keeps the previous
connection pool in use. After a successful refresh, the previous pool is closed,
and connections that are still in use (e.g. by running transactions) are closed
when they are done. Note that pool settings, such as `SetMaxOpenConns`, are not
carried over to the new pool and should be re-applied on `drv.DB()` if needed.

## Use Opencensus With MySQL

```go
package main

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"<project>/ent"

	"contrib.go.opencensus.io/integrations/ocsql"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/go-sql-driver/mysql"
)

type connector struct {
	dsn string
}

func (c connector) Connect(context.Context) (driver.Conn, error) {
	return c.Driver().Open(c.dsn)
}

func (connector) Driver() driver.Driver {
	return ocsql.Wrap(
		mysql.MySQLDriver{},
		ocsql.WithAllTraceOptions(),
		ocsql.WithRowsClose(false),
		ocsql.WithRowsNext(false),
		ocsql.WithDisableErrSkip(true),
	)
}

// Open new connection and start stats recorder.
func Open(dsn string) *ent.Client {
	db := sql.OpenDB(connector{dsn})
	// Create an ent.Driver from `db`.
	drv := entsql.OpenDB(dialect.MySQL, db)
	return ent.NewClient(ent.Driver(drv))
}
```


## Use pgx with PostgreSQL

```go
package main

import (
	"context"
	"database/sql"
	"log"

	"<project>/ent"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open new connection
func Open(databaseUrl string) *ent.Client {
	db, err := sql.Open("pgx", databaseUrl)
	if err != nil {
		log.Fatal(err)
	}

	// Create an ent.Driver from `db`.
	drv := entsql.OpenDB(dialect.Postgres, db)
	return ent.NewClient(ent.Driver(drv))
}

func main() {
	client := Open("postgresql://user:password@127.0.0.1/database")

	// Your code. For example:
	ctx := context.Background()
	if err := client.Schema.Create(ctx); err != nil {
		log.Fatal(err)
	}
	users, err := client.User.Query().All(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(users)
}
```
