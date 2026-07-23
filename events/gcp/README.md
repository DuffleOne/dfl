# events/gcp

Google Cloud transport adapter for the [`events`](../) bus, backed by Pub/Sub
(the v2 SDK, `cloud.google.com/go/pubsub/v2`). Separate module, so the core
stays free of the Pub/Sub SDK: depend on this only in the service that talks
to GCP.

```
go get github.com/duffleone/dfl/events/gcp
```

One transport, two delivery modes, both implementing `events.Sink` so the
rest of your code is identical:

- **`NewPullSink`**: a worker. A background streaming pull receives messages,
  acking on success and nacking on failure. `Subscribe` records the handlers;
  `Receive` starts a pull per subscribed event and blocks until ctx ends.
- **`NewPushSink`**: an `http.Handler`. Pub/Sub push subscriptions POST
  deliveries to it; it dispatches to your handlers and acks with a 2xx. Good
  for serverless, where there's no long-running process to pull.

Both publish to a topic per event name and block on the publish ack, so
`Emit` returns only once Pub/Sub has the message. Publisher handles are
cached per topic, one batching pipeline per event name.

## Quick start (push)

```go
client, _ := pubsub.NewClient(ctx, projectID) // cloud.google.com/go/pubsub/v2
sink := pubsub.NewPushSink(client)

bus := events.NewBus(sink)
bus.On(func(ctx context.Context, e UserCreated) error { /* ... */ return nil })

mux.Handle("POST /events/pubsub", sink) // point a push subscription here
```

Pull is the same shape, with a receiver loop instead of an endpoint. The
consumer name picks the subscriptions: event `user.created` pulls from
subscription `user.created.welcome-service`.

```go
sink := pubsub.NewPullSink(client, "welcome-service")
bus := events.NewBus(sink)
bus.On(welcome)
log.Fatal(sink.Receive(ctx)) // blocking worker loop, started after handlers are registered
```

Runnable examples in [`examples/pull`](./examples/pull) and
[`examples/push`](./examples/push).

## Notes

- Needs Go 1.27 or later: the core uses generic methods, added in that
  release.
- Topics and subscriptions are not created by the sink; provision them out of
  band (terraform, gcloud) with whatever retry and dead-letter policy suits.
- The push ingress does not verify the request's OIDC token. Verify it in
  production so only Pub/Sub can deliver to the endpoint.
