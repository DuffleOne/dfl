# dfl

Personal monorepo of small Go libraries I reuse across the companies and places I work.

## Packages

### [`http`](./http)

Typed HTTP handlers on top of `net/http`, with structured errors and a pluggable mux.

A handler has shape `func(context.Context, *Req) (*Resp, error)`. The router binds `Req` from path, query, and JSON body via struct tags, calls the handler, then JSON-encodes `Resp` on success or runs the error through a `Coercer` on failure. `Req` and `Resp` are pointer-to-struct so handlers return `(nil, err)` cleanly on the error path. `dflhttp.Empty` (and `*dflhttp.Empty`) covers no-input and no-output routes; an `Empty` resp produces `204 No Content`.

The `Router` wraps any mux that satisfies `MethodFunc(method, pattern, handler)` (chi-style) or `HandleFunc(pattern, handler)` (stdlib `"METHOD /path"`-style). Both `*http.ServeMux` and `*chi.Mux` work directly.

```go
type GetUserReq struct {
    ID string `path:"id"`
}

type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

func handleGet(_ context.Context, req *GetUserReq) (*User, error) {
    user, ok := store[req.ID]
    if !ok {
        return nil, dflhttp.New(http.StatusNotFound, "user_not_found", dflhttp.M{"id": req.ID})
    }
    return &user, nil
}

func main() {
    r := dflhttp.NewRouter(http.NewServeMux())
    r.Handle(http.MethodGet, "/users/{id}", handleGet)
    log.Fatal(http.ListenAndServe(":8080", r))
}
```

Working examples in [`http/examples/std`](./http/examples/std) and [`http/examples/chi`](./http/examples/chi). [`http/examples/api/widgets.go`](./http/examples/api/widgets.go) shows multi-source request validation (path + query + body) returning a single 400 with field-level errors.

Optional `Coercer` for `samber/oops` errors lives at [`http/oops`](./http/oops).

### [`db/pgxdb`](./db/pgxdb)

Wrapper around `jackc/pgx/v5`. Transaction shapes (read-only, read-committed, serializable with retry), generic `Get`/`Scalar`/`Select` scanners, and an escape hatch to `*database/sql`. The `Querier` interface is satisfied by both the pool and `pgx.Tx`, so the same helper functions work inside or outside a transaction.

`TxCtx`, `TxReadCtx`, and `TxSerializableCtx` attach the running transaction to the context; helpers can then pull it back out with `GetQuerier(ctx, fallback)`, so a repository function takes a `Querier` once and quietly upgrades to the running tx when called inside a `TxCtx` block. Working examples in [`db/pgxdb/examples/basic`](./db/pgxdb/examples/basic), [`db/pgxdb/examples/tx`](./db/pgxdb/examples/tx), and [`db/pgxdb/examples/serializable`](./db/pgxdb/examples/serializable).
