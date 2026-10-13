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
  the message for redelivery on failure, and `ReceiveOnce` is one turn of
  that for a host with a loop of its own. One queue carries every event; the
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

`Receive` is the loop: it polls until `ctx` ends, and a failed poll is logged
and backed off before the next one.

Push delivery (SNS/EventBridge) mounts the sink on any mux:

```go
sink := sns.NewPushSink(awssns.NewFromConfig(cfg), map[string]string{
    "user.created": topicARN,
})
bus := events.NewBus(sink)
bus.On(welcome)

mux.Handle("POST /events/sns", sink) // SNS posts here
```

Runnable examples in [`examples/sqs`](./examples/sqs),
[`examples/poller`](./examples/poller) and [`examples/sns`](./examples/sns).

## The SQS worker

### Driving it yourself

`Receive` wants a goroutine to itself. A worker that already has a loop, say
a poller that reports health each turn, calls `ReceiveOnce` from it instead:

```go
for ctx.Err() == nil {
    n, err := sink.ReceiveOnce(ctx) // one long poll
    health.Report("events", n, err)

    if err != nil {
        pause(ctx, 5*time.Second) // the wait after a failed poll is yours
    }
}
```

One call is one long poll: every message dispatched, then deleted or left for
redelivery exactly as `Receive` would. It returns how many messages the poll
brought back, 0 for an empty one. That count includes messages whose handler
failed; those reach the bus `ErrorHandler`, not this return. A failed poll
returns its error unlogged and doesn't wait, so the host decides both.

Give each turn a context that outlives the long poll. One that expires
mid-poll comes back as `ctx.Err()`, which reads like a failure on a queue
that was only quiet.

### Options

| Option              | Default               | What it sets                                            |
| ------------------- | --------------------- | ------------------------------------------------------- |
| `WithLongPoll(s)`   | 20                    | Seconds a poll waits for a message, 0 to 20             |
| `WithBatchSize(n)`  | 10                    | Most messages one poll brings back, 1 to 10             |
| `WithBackoff(fn)`   | 1s doubling to 30s    | `Receive`'s wait after a failed poll                    |
| `WithLogger(l)`     | `slog.Default()`      | Where the receive side logs                             |

`WithBackoff` takes the same `func(attempt int) time.Duration` as
`events.RetryPolicy.Backoff`, called with the number of failed polls in a row
and reset by a good one, so the core's helpers fit and so does your own (with
jitter, if a fleet of workers shouldn't retry in step):

```go
sqs.WithBackoff(events.FixedBackoff(5 * time.Second))
```

Without `WithLogger` the sink reads `slog.Default()` each time it logs, so a
host that calls `slog.SetDefault` after building the sink is still honoured.
A handler failure that leaves the message for redelivery logs at warn, the
bus `ErrorHandler` having had the error already; a failed poll or delete logs
at error.

### Stopping

When `ctx` ends mid-batch, the handler that's running sees it end and
returns in its own time. If it succeeds, its message is still deleted, on a
context of its own with a 5s deadline, since a handled message left behind
would only be handled again. The rest of the batch isn't dispatched and comes
back after the queue's visibility timeout. `Receive` returns `ctx.Err()`; a
`ReceiveOnce` cut short returns it with the number of messages it got
through.

### Concurrency

A poll handles its batch one message at a time. The sink keeps nothing
between polls, so `Receive` and `ReceiveOnce` are safe to call from several
goroutines at once, and that is how a worker handles messages in parallel:
run as many loops as you want messages in flight.

The messages at the back of a batch wait for the ones in front, and their
visibility timeout is already running. If handlers are slow, bring
`WithBatchSize` down until a whole batch fits inside it, or the queue
redelivers a message this worker is still about to handle.

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
