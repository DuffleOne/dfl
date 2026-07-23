# db/pgxdb

A thin wrapper around `jackc/pgx/v5`: transaction shapes, generic row
scanning, and a context-carried transaction so repository code doesn't
thread `pgx.Tx` through every signature.

```go
import "github.com/duffleone/dfl/db/pgxdb"

db, err := pgxdb.New(ctx, os.Getenv("DATABASE_URL")) // pools + pings
defer db.Close()
```

## Querier

```go
type Querier interface {
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
```

Both `*pgxdb.DB` (the pool) and `pgx.Tx` satisfy it, so every helper in the
package, and every repository function you write against it, works the same
inside or outside a transaction. This one interface is what the rest of the
package hangs off.

## Reading rows

Three generic helpers cover the usual shapes:

```go
user, err := pgxdb.Get[User](ctx, db,      // exactly one struct row
    `SELECT id, name, email FROM users WHERE id = $1`, id)

n, err := pgxdb.Scalar[int](ctx, db,       // exactly one value
    `SELECT count(*) FROM users`)

all, err := pgxdb.Select[User](ctx, db,    // zero or more struct rows
    `SELECT id, name, email FROM users ORDER BY id`)
```

`Get` and `Select` match columns to struct fields by `db` tag, or lowercased
field name when untagged. `Scalar` takes anything pgx can scan: ints,
strings, `time.Time`, `uuid.UUID`, `sql.Scanner` implementations. `Get` and
`Scalar` return `pgxdb.NotFound` on zero rows (an alias of `pgx.ErrNoRows`,
so `errors.Is` matches either name) and an error on more than one.
`Select` returns an empty slice, not an error, for no rows.

`RETURNING` makes the write helpers unnecessary:

```go
user, err := pgxdb.Get[User](ctx, db,
    `INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, name, email`,
    name, email)
```

## Transactions

Three shapes, each a closure that commits on nil and rolls back on error or
panic:

```go
db.Tx(ctx, f)             // read committed
db.TxRead(ctx, f)         // repeatable read, read-only
db.TxSerializable(ctx, f) // serializable, retried on serialization failure
```

`TxSerializable` retries the closure up to three times when postgres reports
a serialization failure, logging a warning between attempts. The closure
must therefore be safe to re-run: pure reads and writes are fine, but don't
send emails or publish events from inside one. Any other error, or a
cancelled context, returns immediately.

## The context-carried transaction

The `f func(tx pgx.Tx) error` shape forces every function called inside the
transaction to accept the `tx`. The `Ctx` variants attach the transaction to
the context instead:

```go
db.TxCtx(ctx, func(ctx context.Context) error {
    if _, err := repo.Create(ctx, "Alice", "alice@example.com"); err != nil {
        return err
    }

    _, err := repo.Create(ctx, "Bob", "bob@example.com")

    return err // both inserts in one transaction, or neither
})
```

Repository functions then resolve their `Querier` with `GetQuerier`, which
returns the running transaction when there is one and the fallback
otherwise:

```go
func (r *UsersRepo) Create(ctx context.Context, name, email string) (User, error) {
    q := pgxdb.GetQuerier(ctx, r.db)

    return pgxdb.Get[User](ctx, q,
        `INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id, name, email`,
        name, email)
}
```

The same method now runs against the pool when called normally and joins the
ambient transaction when called inside a `TxCtx` block, with no change at
the call site. `TxReadCtx` and `TxSerializableCtx` complete the family (each
serializable retry gets a fresh transaction on a fresh derived context), and
`TxFromContext` exposes the raw `pgx.Tx` for the rare case that needs it.

## Escape hatch

`db.Std()` returns a `*sql.DB` sharing the same pool, plus a cleanup
function, for libraries that only speak `database/sql` (migration runners,
mostly).

## With the http package

`NotFound` is the natural partner of a 404 mapping, either per call site or
once in a `Coercer`:

```go
user, err := users.Get(ctx, db, req.ID)
if errors.Is(err, pgxdb.NotFound) {
    return nil, dflhttp.New(http.StatusNotFound, "user_not_found", dflhttp.M{"id": req.ID})
}
```

## Examples

Each needs a `DATABASE_URL`:

- [`examples/basic`](./examples/basic): the helpers against the pool
- [`examples/tx`](./examples/tx): a repository joining an ambient `TxCtx` transaction
- [`examples/serializable`](./examples/serializable): serializable retries under contention
- [`examples/users`](./examples/users): the shared repository they all use
