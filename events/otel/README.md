# events/otel

OpenTelemetry plugin for the [`events`](../) bus: trace context propagation and
a span on every publish and delivery. Separate module so the core stays free of
the OTel dependency.

```
go get github.com/duffleone/dfl/events/otel
```

Install it with one `WithPlugins` call:

```go
bus := events.NewBus(sink, events.WithPlugins(otel.New()))
```

On **publish** it starts a producer span and injects the trace context into
`Envelope.Headers`. On **deliver** it extracts that context and starts a
consumer span as its child. So a trace started by the emitter flows across the
transport and into the handler, linked by trace id, with span ids per hop.

By default it uses the globally registered `TracerProvider` and
`TextMapPropagator`, so it picks up whatever your app configured with
`otel.SetTracerProvider` / `otel.SetTextMapPropagator`. Override per-bus with
`otel.New(otel.WithTracerProvider(tp), otel.WithPropagator(p))`.

Cross-process propagation needs the sink to carry `Envelope.Headers` over its
transport. `MemSink` does; the `events/aws` (SQS, SNS) and `events/gcp` (Pub/Sub)
adapters map them to native message attributes.

Runnable example with a stdout exporter in [`examples/trace`](./examples/trace).

## Notes

- `go.sum` is not checked in. The core targets a Go with generic methods the
  current toolchain doesn't parse, so `go mod tidy` can't run yet. Once the core
  builds, run `go mod tidy` here to populate `go.sum`.
