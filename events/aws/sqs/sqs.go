// Package sqs provides an events.Sink backed by Amazon SQS: Publish sends
// to the queue, and Receive long-polls it as a worker, dispatching each
// message by the event name in its "event" attribute and deleting it on
// success; ReceiveOnce is one turn of that loop, for a host with its own.
// One queue carries every event type. SQS has no HTTP push; for push see
// sns, eventbridge, or events/gcp/pubsub.
package sqs

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/duffleone/dfl/events"
)

const (
	// eventAttr is the SQS message attribute carrying the event name.
	eventAttr = "event"
	// headersAttr carries the envelope headers as a JSON object, so trace
	// context set by a plugin survives the round trip.
	headersAttr = "headers"

	// Unless WithBackoff says otherwise, Receive waits this long after a
	// failed poll, doubling each time up to the ceiling.
	backoffBase    = time.Second
	backoffCeiling = 30 * time.Second

	// deleteTimeout bounds the delete of a handled message, which runs on a
	// context detached from the caller's.
	deleteTimeout = 5 * time.Second
)

// API is the slice of *sqs.Client this sink calls. Declaring it as an interface
// keeps the sink testable with a fake.
type API interface {
	SendMessage(ctx context.Context, in *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// Sink is an events.Sink over a single SQS queue. Subscribe (from the embedded
// Dispatcher) registers handlers; Publish sends; Receive, or ReceiveOnce a
// poll at a time, pulls and routes.
type Sink struct {
	*events.Dispatcher

	client   API
	queueURL string
	waitSecs int32
	batch    int32
	logger   *slog.Logger
	backoff  func(attempt int) time.Duration
}

var _ events.Sink = (*Sink)(nil)

// Poll is what one ReceiveOnce did with the messages it received. Handled
// ran without error; Failed had a handler error and were left for
// redelivery. Undeleted counts handled messages whose delete then failed,
// so they will come round again. When ctx ends mid-batch, Received runs
// ahead of Handled plus Failed by the messages never dispatched.
type Poll struct {
	Received  int
	Handled   int
	Failed    int
	Undeleted int
}

// Option configures a Sink.
type Option func(*Sink)

// WithLongPoll sets the receive wait time in seconds (0-20). Defaults to 20.
func WithLongPoll(seconds int32) Option {
	return func(s *Sink) { s.waitSecs = seconds }
}

// WithBatchSize sets the max messages pulled per receive (1-10). Defaults to 10.
func WithBatchSize(n int32) Option {
	return func(s *Sink) { s.batch = n }
}

// WithLogger sets where the receive side logs. Unset, the sink uses
// slog.Default() as it logs, so a host that calls slog.SetDefault after
// building the sink is still honoured.
func WithLogger(l *slog.Logger) Option {
	return func(s *Sink) { s.logger = l }
}

// WithBackoff sets how long Receive waits after a failed poll. It's called
// with the number of failures in a row (1 for the first), a good poll
// resetting the count, and has the shape of events.RetryPolicy.Backoff, so
// events.ExponentialBackoff and events.FixedBackoff both fit. Defaults to 1s
// doubling to a 30s ceiling. ReceiveOnce never waits; its host decides.
func WithBackoff(backoff func(attempt int) time.Duration) Option {
	return func(s *Sink) { s.backoff = backoff }
}

// NewSink builds a Sink for the queue at queueURL.
func NewSink(client API, queueURL string, opts ...Option) *Sink {
	s := &Sink{
		Dispatcher: events.NewDispatcher(),
		client:     client,
		queueURL:   queueURL,
		waitSecs:   20,
		batch:      10,
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.backoff == nil {
		s.backoff = defaultBackoff
	}

	return s
}

// defaultBackoff is 1s doubling to a 30s ceiling.
func defaultBackoff(attempt int) time.Duration {
	return min(events.ExponentialBackoff(backoffBase)(attempt), backoffCeiling)
}

// log returns the configured logger, or the process default as of now.
func (s *Sink) log() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}

	return slog.Default()
}

// Publish sends env to the queue, tagging the event name so Receive can route
// it. SendMessage returns once SQS has durably accepted the message, which is
// the guarantee Emit relies on.
func (s *Sink) Publish(ctx context.Context, env events.Envelope) error {
	attrs := map[string]types.MessageAttributeValue{
		eventAttr: {DataType: aws.String("String"), StringValue: aws.String(env.Name)},
	}

	if h := marshalHeaders(env.Headers); h != "" {
		attrs[headersAttr] = types.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(h)}
	}

	_, err := s.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(s.queueURL),
		MessageBody:       aws.String(string(env.Payload)),
		MessageAttributes: attrs,
	})
	if err != nil {
		return events.Wrap(err, "publish_failed", events.M{"event": env.Name})
	}

	return nil
}

// Receive drives ReceiveOnce until ctx ends, which is the only way it
// returns. A failed poll is logged and backed off (see WithBackoff) rather
// than retried hot. Call it after the bus has registered its handlers, as
// the worker's main loop or a goroutine; a worker that wants to handle
// messages in parallel runs several.
func (s *Sink) Receive(ctx context.Context) error {
	var failures int

	for {
		_, err := s.ReceiveOnce(ctx)
		if err == nil {
			failures = 0

			continue
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		failures++
		delay := s.backoff(failures)

		s.log().ErrorContext(ctx, "sqs: receive failed",
			slog.String("error", err.Error()), slog.Duration("backoff", delay))

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

// ReceiveOnce is one long poll (the configured wait and batch size), each
// message dispatched to its handler and deleted on success or left for
// redelivery on failure. The Poll it returns says what became of them, so a
// host running its own loop can report on each turn; if ctx ends mid-batch
// the rest stay queued and the Poll so far comes back with ctx.Err(). A
// failed poll returns its error rather than logging it.
func (s *Sink) ReceiveOnce(ctx context.Context) (Poll, error) {
	if err := ctx.Err(); err != nil {
		return Poll{}, err
	}

	out, err := s.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(s.queueURL),
		MaxNumberOfMessages:   s.batch,
		WaitTimeSeconds:       s.waitSecs,
		MessageAttributeNames: []string{eventAttr, headersAttr},
	})
	if err != nil {
		if ctx.Err() != nil {
			return Poll{}, ctx.Err()
		}

		return Poll{}, err
	}

	poll := Poll{Received: len(out.Messages)}

	for _, m := range out.Messages {
		if err := ctx.Err(); err != nil {
			return poll, err
		}

		s.handle(ctx, m, &poll)
	}

	return poll, nil
}

// handle dispatches one message and deletes it if that worked, counting the
// outcome on poll.
func (s *Sink) handle(ctx context.Context, m types.Message, poll *Poll) {
	var name, body string

	if a, ok := m.MessageAttributes[eventAttr]; ok && a.StringValue != nil {
		name = *a.StringValue
	}

	if m.Body != nil {
		body = *m.Body
	}

	var headers map[string]string
	if a, ok := m.MessageAttributes[headersAttr]; ok && a.StringValue != nil {
		headers = unmarshalHeaders(*a.StringValue)
	}

	env := events.Envelope{Name: name, Payload: []byte(body), Headers: headers}

	if err := s.Dispatch(ctx, env); err != nil {
		poll.Failed++

		// Leave the message; SQS redelivers after the visibility timeout. The
		// bus ErrorHandler has had the error already, so this is a warning.
		s.log().WarnContext(ctx, "sqs: handler failed, leaving for redelivery",
			slog.String("event", name), slog.String("error", err.Error()))

		return
	}

	poll.Handled++

	// The handler is done, so the delete goes out even if ctx ended while it
	// ran (a shutdown, say): skip it and the message is redelivered and
	// handled twice. Detached from ctx, it gets a deadline of its own.
	deleteCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), deleteTimeout)
	defer cancel()

	if _, err := s.client.DeleteMessage(deleteCtx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(s.queueURL),
		ReceiptHandle: m.ReceiptHandle,
	}); err != nil {
		poll.Undeleted++

		s.log().ErrorContext(ctx, "sqs: delete failed",
			slog.String("event", name), slog.String("error", err.Error()))
	}
}

// marshalHeaders encodes envelope headers to a JSON string for transport, or ""
// when there are none.
func marshalHeaders(h map[string]string) string {
	if len(h) == 0 {
		return ""
	}

	b, err := json.Marshal(h)
	if err != nil {
		return ""
	}

	return string(b)
}

// unmarshalHeaders decodes the JSON string produced by marshalHeaders.
func unmarshalHeaders(s string) map[string]string {
	if s == "" {
		return nil
	}

	var h map[string]string
	if err := json.Unmarshal([]byte(s), &h); err != nil {
		return nil
	}

	return h
}
