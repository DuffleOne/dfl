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
code from its message, and anything else becomes `unknown`.

Everything except the pass-through gets a 500, overriding the status
`ReqError` would otherwise
[derive from the code](../README.md#the-status-comes-from-the-code). An oops
error is one that arrived carrying a stack trace, which makes it something
that went wrong on this side rather than a contract the caller can act on. If
some of your oops codes are the caller's problem, that's a `Coercer` of your
own that checks for them first and falls back to `oops.Coerce`.

See [`http/examples/chi`](../examples/chi) for it wired into a running
service.
