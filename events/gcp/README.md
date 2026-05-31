# events/gcp

Google Cloud transport adapter for the [`events`](../) bus, backed by Pub/Sub.
Separate module, so the core stays free of the Pub/Sub SDK: depend on this only
in the service that talks to GCP.

```
go get github.com/duffleone/dfl/events/gcp
```

One transport, two delivery modes, both implementing `events.Sink` so the rest
of your code is identical:

- **`NewPullSink`** — a worker. A background streaming pull receives messages,
  acking on success and nacking on failure. Run it and `select{}` (or do other
  work); `Subscribe` starts the receiver per event.
- **`NewPushSink`** — an `http.Handler`. Pub/Sub push subscriptions POST
  deliveries to it; it dispatches to your handlers and acks with a 2xx. Good for
  serverless, where there's no long-running process to pull.

Both publish to a topic per event name and block on the publish ack, so `Emit`
returns only once Pub/Sub has the message.

## Quick start (push)

```go
client, _ := pubsub.NewClient(ctx, projectID)
sink := pubsub.NewPushSink(client)

bus := events.NewBus(sink)
bus.On(func(ctx context.Context, e UserCreated) error { /* ... */ return nil })

mux.Handle("POST /events/pubsub", sink) // point a push subscription here
```

Pull is the same shape, with a receiver loop instead of an endpoint:

```go
sink := pubsub.NewPullSink(ctx, client, "welcome-service")
bus := events.NewBus(sink)
bus.On(welcome)
// Subscribe started the receiver; keep the process alive.
```

Runnable examples in [`examples/pull`](./examples/pull) and
[`examples/push`](./examples/push).

## Notes

- `go.sum` is not checked in. The core targets a Go with generic methods the
  current toolchain doesn't parse, so `go mod tidy` can't run yet. Once the core
  builds, run `go mod tidy` here to pin the version and populate `go.sum`.
- The push ingress does not verify the request's OIDC token. Verify it in
  production so only Pub/Sub can deliver to the endpoint.
