# http/cher

A [`Coercer`](../README.md#errors) that understands
[`cher`](https://github.com/wearemojo/mojo-public-go/tree/main/lib/cher)
errors. Install it and handlers can keep returning `cher.E` while the router
does the HTTP part.

It's its own module, because mojo-public-go brings mongo, stripe, grpc, and
the GCP SDKs with it and the core shouldn't inherit that:

```
go get github.com/duffleone/dfl/http/cher
```

It also shares a name with the library it adapts, so alias it wherever you
need both:

```go
import (
    "github.com/wearemojo/mojo-public-go/lib/cher"

    dflhttp "github.com/duffleone/dfl/http"
    dflcher "github.com/duffleone/dfl/http/cher"
)

r := dflhttp.NewRouter(mux, dflhttp.WithCoercer(dflcher.Coerce))
```

```go
func handleGet(ctx context.Context, req *GetTeamReq) (*Team, error) {
    team, err := store.Get(ctx, req.ID)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, cher.New(cher.NotFound, cher.M{"id": req.ID})
    }
    // ...
}
```

```json
{"code": "not_found", "meta": {"id": "42"}}
```

That body is byte-for-byte what cher would have written: `ReqError` carries no
status of its own, so a service swapping cher's HTTP layer for this one
doesn't move its clients.

## What maps to what

Coercion order: `nil` passes through, an existing `*ReqError` wins, a `cher.E`
anywhere on the error chain gets projected, and anything else becomes
`unknown`, which is a 500.

**Status** comes from cher's own `StatusCode()`, set explicitly rather than
left to `ReqError`'s table. The two mostly agree and both default to 400, but
cher's is the contract a cher-shaped service already publishes, so it wins
here and stays right if either drifts. Concretely: 401 `unauthorized`, 403
`access_denied`, 404 `not_found` and `route_not_found`, 405, 410
`endpoint_withdrawn`, 429, and 500 for `unknown`, `unable_to_coerce_error`,
and `request_timeout`. Everything else is a 400, your own service-specific
codes included.

**Meta** carries over key for key, copied rather than shared, so re-wrapping
the cher error later can't mutate a response body that already went out.

**Reasons** keep their tree. Both sides nest, so a reason's children stay
under it:

```go
cher.New(cher.BadRequest, nil,
    cher.New("required", cher.M{"field": "name"}),
    cher.New("invalid", cher.M{"field": "size"},
        cher.New("too_small", cher.M{"min": 1}),
    ),
)
```

```json
{"code": "bad_request",
 "reasons": [{"code": "required", "meta": {"field": "name"}},
             {"code": "invalid", "meta": {"field": "size"},
              "reasons": [{"code": "too_small", "meta": {"min": 1}}]}]}
```

## Two deliberate omissions

An error that isn't a `cher.E` becomes a bare 500 `unknown` with no meta, and
its message stays on the cause chain for your logs. This is where the coercer
parts company with `cher.Coerce`, which would put `err.Error()` in the meta:
an error that got this far was never classified, so its text is as likely to
be a driver message or a file path as something a caller should read.

`cher.E`'s `Extra` field is dropped. It holds whatever unrecognised keys an
upstream service's error body carried, kept for log forensics, and forwarding
those would leak one service's error shape through another.

## What still differs from cher

The body matches, but a `cher.E` that came off the wire from an upstream
service may carry `_extra`, and this drops it (above). If you need cher's
bytes reproduced exactly, including that, an
[`ErrorWriter`](../README.md#3-errorwriter-owning-the-response-outright) is
the layer for it: `cher.E` marshals to its own shape directly, and the writer
never consults a `Coercer`.
