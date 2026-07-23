# http/oops

A [`Coercer`](../README.md#errors) that understands
[`samber/oops`](https://github.com/samber/oops) errors. Opt-in: install it
and the router projects oops codes, context, and public messages into
`ReqError` meta instead of collapsing them to 500 `unknown`.

```go
r := dflhttp.NewRouter(mux, dflhttp.WithCoercer(oops.Coerce))
```

Coercion order: nil passes through, an existing `*ReqError` wins, an oops
error contributes its code and context, a wrapped generic error derives a
code from its message, and anything else becomes 500 `unknown`. See
[`http/examples/chi`](../examples/chi) for it wired into a running service.
