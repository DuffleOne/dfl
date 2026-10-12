package sqs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/duffleone/dfl/events"
	"github.com/duffleone/dfl/events/aws/sqs"
)

// fakeAPI is a scripted stand-in for the SQS client. SendMessage and
// DeleteMessage record their inputs; ReceiveMessage fails with each of errs
// first, then pops one batch per call and, once the script runs dry, blocks
// until ctx is cancelled, which is how a test tells the receive loop to stop.
// polls records when each ReceiveMessage call arrived.
type fakeAPI struct {
	sent    []*awssqs.SendMessageInput
	errs    []error
	batches [][]types.Message
	polls   []time.Time
	deleted []string
}

func (f *fakeAPI) SendMessage(_ context.Context, in *awssqs.SendMessageInput, _ ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error) {
	f.sent = append(f.sent, in)

	return &awssqs.SendMessageOutput{}, nil
}

func (f *fakeAPI) ReceiveMessage(ctx context.Context, _ *awssqs.ReceiveMessageInput, _ ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
	f.polls = append(f.polls, time.Now())

	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]

		return nil, err
	}

	if len(f.batches) == 0 {
		<-ctx.Done()

		return nil, ctx.Err()
	}

	batch := f.batches[0]
	f.batches = f.batches[1:]

	return &awssqs.ReceiveMessageOutput{Messages: batch}, nil
}

func (f *fakeAPI) DeleteMessage(_ context.Context, in *awssqs.DeleteMessageInput, _ ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error) {
	f.deleted = append(f.deleted, *in.ReceiptHandle)

	return &awssqs.DeleteMessageOutput{}, nil
}

// message builds a queue message the way Publish would have tagged it.
func message(name, body, receipt string) types.Message {
	return types.Message{
		Body:          aws.String(body),
		ReceiptHandle: aws.String(receipt),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"event": {DataType: aws.String("String"), StringValue: aws.String(name)},
		},
	}
}

// receive runs sink.Receive in the background and returns a stop function
// that cancels it and asserts it returned context.Canceled.
func receive(t *testing.T, sink *sqs.Sink) (stop func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() { done <- sink.Receive(ctx) }()

	return func() {
		cancel()

		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("Receive returned %v, want context.Canceled", err)
		}
	}
}

// TestPublishTagsEventAndHeaders pins the wire contract Receive depends on:
// the event name rides in the "event" message attribute and the envelope
// headers ride in "headers" as JSON.
func TestPublishTagsEventAndHeaders(t *testing.T) {
	fake := &fakeAPI{}
	sink := sqs.NewSink(fake, "https://queue.test/q")

	env := events.Envelope{
		Name:    "user.created",
		Payload: []byte(`{"id":"1"}`),
		Headers: map[string]string{"traceparent": "00-abc"},
	}

	if err := sink.Publish(t.Context(), env); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if len(fake.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(fake.sent))
	}

	in := fake.sent[0]

	if got := aws.ToString(in.QueueUrl); got != "https://queue.test/q" {
		t.Errorf("queue url = %q, want the configured queue", got)
	}

	if got := aws.ToString(in.MessageBody); got != `{"id":"1"}` {
		t.Errorf("body = %q, want the raw payload", got)
	}

	if got := aws.ToString(in.MessageAttributes["event"].StringValue); got != "user.created" {
		t.Errorf("event attribute = %q, want user.created", got)
	}

	var headers map[string]string
	if err := json.Unmarshal([]byte(aws.ToString(in.MessageAttributes["headers"].StringValue)), &headers); err != nil {
		t.Fatalf("headers attribute is not JSON: %v", err)
	}

	if headers["traceparent"] != "00-abc" {
		t.Errorf("headers = %v, want the traceparent carried over", headers)
	}
}

// TestReceiveDispatchesAndDeletes: a handled message reaches the subscribed
// handler with its payload intact and is then deleted from the queue.
func TestReceiveDispatchesAndDeletes(t *testing.T) {
	fake := &fakeAPI{batches: [][]types.Message{{message("user.created", `{"id":"1"}`, "r-1")}}}
	sink := sqs.NewSink(fake, "https://queue.test/q")

	got := make(chan events.Envelope, 1)

	sink.Subscribe("user.created", func(_ context.Context, env events.Envelope) error {
		got <- env

		return nil
	})

	stop := receive(t, sink)

	env := <-got
	if string(env.Payload) != `{"id":"1"}` {
		t.Errorf("payload = %s, want the message body", env.Payload)
	}

	stop()

	if len(fake.deleted) != 1 || fake.deleted[0] != "r-1" {
		t.Errorf("deleted = %v, want [r-1]", fake.deleted)
	}
}

// TestReceiveLeavesFailedMessages: a handler error must not delete the
// message, so SQS redelivers it after the visibility timeout.
func TestReceiveLeavesFailedMessages(t *testing.T) {
	fake := &fakeAPI{batches: [][]types.Message{{message("user.created", `{"id":"1"}`, "r-1")}}}
	sink := sqs.NewSink(fake, "https://queue.test/q")

	handled := make(chan struct{}, 1)

	sink.Subscribe("user.created", func(_ context.Context, _ events.Envelope) error {
		handled <- struct{}{}

		return events.New("boom", nil)
	})

	stop := receive(t, sink)

	<-handled
	stop()

	if len(fake.deleted) != 0 {
		t.Errorf("deleted = %v, want none: a failed message must stay for redelivery", fake.deleted)
	}
}

// TestReceiveOnceReportsThePoll: one call is one poll, and what comes back
// describes it. A failed poll returns its error, an empty one 0, and a batch
// its size with every message handled and deleted.
func TestReceiveOnceReportsThePoll(t *testing.T) {
	boom := errors.New("boom")
	fake := &fakeAPI{
		errs: []error{boom},
		batches: [][]types.Message{
			{},
			{message("user.created", `{"id":"1"}`, "r-1"), message("user.created", `{"id":"2"}`, "r-2")},
		},
	}
	sink := sqs.NewSink(fake, "https://queue.test/q")

	var handled int

	sink.Subscribe("user.created", func(_ context.Context, _ events.Envelope) error {
		handled++

		return nil
	})

	if n, err := sink.ReceiveOnce(t.Context()); n != 0 || !errors.Is(err, boom) {
		t.Errorf("failed poll returned (%d, %v), want (0, boom)", n, err)
	}

	if n, err := sink.ReceiveOnce(t.Context()); n != 0 || err != nil {
		t.Errorf("empty poll returned (%d, %v), want (0, nil)", n, err)
	}

	if n, err := sink.ReceiveOnce(t.Context()); n != 2 || err != nil {
		t.Errorf("poll of two returned (%d, %v), want (2, nil)", n, err)
	}

	if handled != 2 {
		t.Errorf("handled %d messages, want 2", handled)
	}

	if len(fake.deleted) != 2 {
		t.Errorf("deleted = %v, want both receipts", fake.deleted)
	}
}

// TestReceiveOnceStopsWhenContextEnds: a ctx that ends mid-batch stops the
// dispatching there, the count so far comes back with ctx.Err(), and the
// messages not reached stay on the queue. A dead ctx doesn't poll at all.
func TestReceiveOnceStopsWhenContextEnds(t *testing.T) {
	fake := &fakeAPI{batches: [][]types.Message{{
		message("user.created", `{"id":"1"}`, "r-1"),
		message("user.created", `{"id":"2"}`, "r-2"),
		message("user.created", `{"id":"3"}`, "r-3"),
	}}}
	sink := sqs.NewSink(fake, "https://queue.test/q")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var handled int

	sink.Subscribe("user.created", func(_ context.Context, _ events.Envelope) error {
		handled++

		cancel() // the first message pulls the plug

		return nil
	})

	n, err := sink.ReceiveOnce(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ReceiveOnce returned %v, want context.Canceled", err)
	}

	if n != 1 || handled != 1 {
		t.Errorf("count %d, handled %d, want 1 and 1: dispatching should stop once ctx ends", n, handled)
	}

	if len(fake.deleted) != 1 || fake.deleted[0] != "r-1" {
		t.Errorf("deleted = %v, want [r-1]: the rest should stay on the queue", fake.deleted)
	}

	if n, err := sink.ReceiveOnce(ctx); n != 0 || !errors.Is(err, context.Canceled) {
		t.Errorf("ReceiveOnce on a dead ctx returned (%d, %v), want (0, context.Canceled)", n, err)
	}

	if len(fake.polls) != 1 {
		t.Errorf("polled %d times, want 1: a dead ctx shouldn't reach SQS", len(fake.polls))
	}
}

// TestReceiveBacksOffOnReceiveErrors: a failed poll pushes the next one back,
// a second failure twice as far, and the loop carries on to the message after
// them. Only lower bounds are checked; a sleep is never shorter than asked.
func TestReceiveBacksOffOnReceiveErrors(t *testing.T) {
	boom := errors.New("boom")
	fake := &fakeAPI{
		errs:    []error{boom, boom},
		batches: [][]types.Message{{message("user.created", `{"id":"1"}`, "r-1")}},
	}
	sink := sqs.NewSink(fake, "https://queue.test/q", sqs.WithBackoff(20*time.Millisecond, time.Second))

	handled := make(chan struct{}, 1)

	sink.Subscribe("user.created", func(_ context.Context, _ events.Envelope) error {
		handled <- struct{}{}

		return nil
	})

	stop := receive(t, sink)

	<-handled
	stop()

	if len(fake.polls) < 3 {
		t.Fatalf("polled %d times, want at least 3: two failures and the batch", len(fake.polls))
	}

	if gap := fake.polls[1].Sub(fake.polls[0]); gap < 20*time.Millisecond {
		t.Errorf("second poll came %v after the first, want at least the 20ms backoff", gap)
	}

	if gap := fake.polls[2].Sub(fake.polls[1]); gap < 40*time.Millisecond {
		t.Errorf("third poll came %v after the second, want at least the doubled 40ms", gap)
	}
}

// TestWithLoggerHearsTheNack: a handler failure leaves the message and says
// so on the configured logger at warn, not error, since the bus ErrorHandler
// has had the error already.
func TestWithLoggerHearsTheNack(t *testing.T) {
	var buf bytes.Buffer

	fake := &fakeAPI{batches: [][]types.Message{{message("user.deleted", `{"id":"1"}`, "r-1")}}}
	sink := sqs.NewSink(fake, "https://queue.test/q", sqs.WithLogger(slog.New(slog.NewTextHandler(&buf, nil))))

	sink.Subscribe("user.deleted", func(_ context.Context, _ events.Envelope) error {
		return events.New("boom", nil)
	})

	if n, err := sink.ReceiveOnce(t.Context()); n != 1 || err != nil {
		t.Fatalf("ReceiveOnce returned (%d, %v), want (1, nil): a nack is not a failed poll", n, err)
	}

	line := buf.String()

	for _, want := range []string{"level=WARN", `msg="sqs: handler failed, leaving for redelivery"`, "event=user.deleted"} {
		if !strings.Contains(line, want) {
			t.Errorf("log = %q, want it to contain %q", line, want)
		}
	}

	if len(fake.deleted) != 0 {
		t.Errorf("deleted = %v, want none", fake.deleted)
	}
}
