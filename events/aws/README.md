# events/aws

AWS transport adapters for the [`events`](../) bus. Separate module, so the
core stays free of the AWS SDK: depend on this only in the service that talks
to AWS.

```
go get github.com/duffleone/dfl/events/aws
```

Three transports, each implementing the same `events.Sink` shape so the rest
of your code (defining events, `bus.On`, `bus.Emit`) is identical no matter
which you pick:

- **`sqs`**: the basic one, and a complete `Sink`. `Publish` sends to the
  queue; `Receive` long-polls and dispatches, deleting on success and leaving
  the message for redelivery on failure. One queue carries every event; the
  name rides in the `event` message attribute. Pull only.
- **`sns`**: a `Publisher` plus a push `PushSink` that is an `http.Handler`.
  Publish to a topic per event; receive via an SNS HTTP subscription (it
  handles the confirmation handshake) or by subscribing an SQS queue to the
  topic and pulling it with `sqs`.
- **`eventbridge`**: the more complex one. A `Publisher` (`PutEvents`, event
  name as the detail-type) plus a push `PushSink` for an HTTP API destination
  target. Pull via an SQS target.

## Quick start (SQS worker)

```go
cfg, _ := config.LoadDefaultConfig(ctx)
sink := sqs.NewSink(awssqs.NewFromConfig(cfg), queueURL)

bus := events.NewBus(sink)
bus.On(func(ctx context.Context, e UserCreated) error { /* ... */ return nil })

if err := bus.Emit(ctx, UserCreated{ID: "1", Email: "a@b.com"}); err != nil {
    log.Fatal(err)
}

log.Fatal(sink.Receive(ctx)) // blocking worker loop, runs until ctx ends
```

Push delivery (SNS/EventBridge) mounts the sink on any mux:

```go
sink := sns.NewPushSink(awssns.NewFromConfig(cfg), map[string]string{
    "user.created": topicARN,
})
bus := events.NewBus(sink)
bus.On(welcome)

mux.Handle("POST /events/sns", sink) // SNS posts here
```

Runnable examples in [`examples/sqs`](./examples/sqs) and
[`examples/sns`](./examples/sns).

## Notes

- Needs Go 1.27 or later: the core uses generic methods, added in that
  release.
- An SQS message whose event name has nothing subscribed dispatches to no
  handlers and is deleted. A shared queue should only receive the events its
  consumer handles.
- Envelope headers (trace context) ride in the `headers` message attribute on
  SQS and SNS. EventBridge entries have no attribute bag, so headers are not
  carried there; route through SQS if you need trace propagation.
- The SNS push ingress does not verify message signatures. Verify them (and
  pin the topic) before trusting a message in production.
