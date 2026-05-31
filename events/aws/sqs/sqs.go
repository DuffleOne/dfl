// Package sqs provides an events.Sink backed by Amazon SQS, the basic AWS event
// transport: Publish sends a message to the queue, and Receive long-polls it,
// dispatching each message to the handler registered for its event name and
// deleting it on success. One queue carries every event type; each message
// tags its name in the "event" message attribute.
//
// This is a pull sink: you run Receive as a worker. SQS has no HTTP push, so
// there is no push variant here. For push, see the sns and eventbridge packages
// or events/gcp/pubsub.
package sqs

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/duffleone/dfl/events"
)

// eventAttr is the SQS message attribute carrying the event name.
const eventAttr = "event"

// API is the slice of *sqs.Client this sink calls. Declaring it as an interface
// keeps the sink testable with a fake.
type API interface {
	SendMessage(ctx context.Context, in *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
	ReceiveMessage(ctx context.Context, in *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, in *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
}

// Sink is an events.Sink over a single SQS queue. Subscribe (from the embedded
// Dispatcher) registers handlers; Publish sends; Receive pulls and routes.
type Sink struct {
	*events.Dispatcher

	client   API
	queueURL string
	waitSecs int32
	batch    int32
}

var _ events.Sink = (*Sink)(nil)

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

	return s
}

// Publish sends env to the queue, tagging the event name so Receive can route
// it. SendMessage returns once SQS has durably accepted the message, which is
// the guarantee Emit relies on.
func (s *Sink) Publish(ctx context.Context, env events.Envelope) error {
	_, err := s.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.queueURL),
		MessageBody: aws.String(string(env.Payload)),
		MessageAttributes: map[string]types.MessageAttributeValue{
			eventAttr: {DataType: aws.String("String"), StringValue: aws.String(env.Name)},
		},
	})
	if err != nil {
		return events.Wrap(err, "publish_failed", events.M{"event": env.Name})
	}

	return nil
}

// Receive long-polls the queue and dispatches each message to its handler,
// deleting it on success and leaving it (for redelivery after the visibility
// timeout) on failure. It blocks until ctx is cancelled. Call it once, after
// the bus has registered its handlers, as the worker's main loop or a goroutine.
func (s *Sink) Receive(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		out, err := s.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:              aws.String(s.queueURL),
			MaxNumberOfMessages:   s.batch,
			WaitTimeSeconds:       s.waitSecs,
			MessageAttributeNames: []string{eventAttr},
		})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			slog.ErrorContext(ctx, "sqs: receive failed", slog.String("error", err.Error()))

			continue
		}

		for _, m := range out.Messages {
			s.handle(ctx, m)
		}
	}
}

func (s *Sink) handle(ctx context.Context, m types.Message) {
	var name, body string

	if a, ok := m.MessageAttributes[eventAttr]; ok && a.StringValue != nil {
		name = *a.StringValue
	}

	if m.Body != nil {
		body = *m.Body
	}

	env := events.Envelope{Name: name, Payload: []byte(body)}

	if err := s.Dispatch(ctx, env); err != nil {
		// Leave the message; SQS redelivers after the visibility timeout.
		slog.ErrorContext(ctx, "sqs: handler failed, leaving for redelivery",
			slog.String("event", name), slog.String("error", err.Error()))

		return
	}

	if _, err := s.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(s.queueURL),
		ReceiptHandle: m.ReceiptHandle,
	}); err != nil {
		slog.ErrorContext(ctx, "sqs: delete failed", slog.String("error", err.Error()))
	}
}
