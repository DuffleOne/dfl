# dfl

Personal monorepo of small Go libraries I reuse across the companies and places I work.

Packages:

- [`http`](./http): typed handlers over `net/http`, with structured errors and a pluggable backend. Stdlib `ServeMux` backend lives at [`http/std`](./http/std); other routers (e.g. go-chi) plug in via a small adapter at the call site, see [`examples/chi`](./examples/chi).
