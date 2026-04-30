# dfl

Personal monorepo of small Go libraries I reuse across the companies and places I work.

Packages:

- [`http`](./http): typed handlers over `net/http`, with structured errors. One `Router` type wraps any mux that registers handlers per method+pattern (chi-style) or per stdlib `"METHOD /path"` pattern. See [`examples/std`](./examples/std) and [`examples/chi`](./examples/chi).
