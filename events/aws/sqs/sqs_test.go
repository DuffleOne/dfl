package sqs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/duffleone/dfl/events"
	"github.com/duffleone/dfl/events/aws/sqs"
)

// fakeAPI is a scripted stand-in for the SQS client. SendMessage and
// DeleteMessage record their inputs, DeleteMessage refusing a dead ctx as the
// real client does. ReceiveMessage answers with each of errs first (a nil one
// falls through to the batches), then pops a batch per call and, once the
// script runs dry, blocks until ctx is cancelled, which is how a test stops
// the receive loop. polls records when each ReceiveMessage arrived.
type fakeAPI struct {
	mu      sync.Mutex
	sent    []*awssqs.SendMessageInput
	errs    []error
	batches [][]types.Message
	polls   []time.Time
	deleted []string
}

func (f *fakeAPI) SendMessage(_ context.Context, in *awssqs.SendMessageInput, _ ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.sent = append(f.sent, in)

	return &awssqs.SendMessageOutput{}, nil
}

func (f *fakeAPI) ReceiveMessage(ctx context.Context, _ *awssqs.ReceiveMessageInput, _ ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
	batch, ok, err := f.next()
	if err != nil {
		return nil, err
	}

	if !ok {
		<-ctx.Done()

		return nil, ctx.Err()
	}

	return &awssqs.ReceiveMessageOutput{Messages: batch}, nil
}

// next pops the script's next poll: an error, a batch, or ok false once dry.
func (f *fakeAPI) next() (batch []types.Message, ok bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.polls = append(f.polls, time.Now())

	if len(f.errs) > 0 {
		err, f.errs = f.errs[0], f.errs[1:]
		if err != nil {
			return nil, false, err
		}
	}

	if len(f.batches) == 0 {
		return nil, false, nil
	}

	batch, f.batches = f.batches[0], f.batches[1:]

	return batch, true, nil
}

func (f *fakeAPI) DeleteMessage(ctx context.Context, in *awssqs.DeleteMessageInput, _ ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

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
// dispatching there and the count so far comes back with ctx.Err(). The
// message already handled is still deleted, on a context of its own, or it
// would come round again; the ones not reached stay on the queue. A dead ctx
// doesn't poll at all.
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
		t.Errorf("deleted = %v, want [r-1]: a handled message is deleted even though ctx has ended, the rest stay", fake.deleted)
	}

	if n, err := sink.ReceiveOnce(ctx); n != 0 || !errors.Is(err, context.Canceled) {
		t.Errorf("ReceiveOnce on a dead ctx returned (%d, %v), want (0, context.Canceled)", n, err)
	}

	if len(fake.polls) != 1 {
		t.Errorf("polled %d times, want 1: a dead ctx shouldn't reach SQS", len(fake.polls))
	}
}

// gaps returns the time between each poll and the one before it.
func gaps(polls []time.Time) []time.Duration {
	out := make([]time.Duration, 0, len(polls))

	for i := 1; i < len(polls); i++ {
		out = append(out, polls[i].Sub(polls[i-1]))
	}

	return out
}

// TestReceiveBacksOffOnFailedPolls: failed polls wait 1s, doubling to the 30s
// ceiling, a good poll puts the next failure back at 1s, and each failure is
// logged at error. synctest's clock runs the real schedule instantly and
// exactly, so the defaults are what's under test.
func TestReceiveBacksOffOnFailedPolls(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		boom := errors.New("boom")

		var buf bytes.Buffer

		fake := &fakeAPI{
			errs:    []error{boom, boom, boom, boom, boom, boom, boom, nil, boom},
			batches: [][]types.Message{{}},
		}
		sink := sqs.NewSink(fake, "https://queue.test/q", sqs.WithLogger(slog.New(slog.NewTextHandler(&buf, nil))))

		stop := receive(t, sink)

		time.Sleep(time.Hour) // well past the script: the last poll is parked on ctx
		stop()

		want := []time.Duration{
			time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second,
			30 * time.Second, 30 * time.Second, // the ceiling
			0,           // the good poll goes straight round again
			time.Second, // and the failure after it starts over
		}

		if got := gaps(fake.polls); !slices.Equal(got, want) {
			t.Errorf("gaps between polls = %v, want %v", got, want)
		}

		if got := strings.Count(buf.String(), `level=ERROR msg="sqs: receive failed"`); got != 8 {
			t.Errorf("logged %d failed polls at error, want 8:\n%s", got, buf.String())
		}
	})
}

// TestWithBackoffSetsTheWait: the option takes the events.RetryPolicy.Backoff
// shape, is asked with the number of failures in a row, and what it answers
// is how long the loop waits.
func TestWithBackoffSetsTheWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		boom := errors.New("boom")
		fake := &fakeAPI{errs: []error{boom, boom, nil, boom}, batches: [][]types.Message{{}}}

		var asked []int

		sink := sqs.NewSink(fake, "https://queue.test/q",
			sqs.WithLogger(slog.New(slog.DiscardHandler)),
			sqs.WithBackoff(func(attempt int) time.Duration {
				asked = append(asked, attempt)

				return events.FixedBackoff(5 * time.Second)(attempt)
			}),
		)

		stop := receive(t, sink)

		time.Sleep(time.Hour)
		stop()

		if want := []int{1, 2, 1}; !slices.Equal(asked, want) {
			t.Errorf("backoff asked with %v, want %v: a good poll resets the count", asked, want)
		}

		want := []time.Duration{5 * time.Second, 5 * time.Second, 0, 5 * time.Second}
		if got := gaps(fake.polls); !slices.Equal(got, want) {
			t.Errorf("gaps between polls = %v, want %v", got, want)
		}
	})
}

// TestReceiveOnceRunsInParallel: the sink keeps nothing between polls, so
// several loops can share one, which is how a worker handles messages in
// parallel. The race detector is the real assertion here.
func TestReceiveOnceRunsInParallel(t *testing.T) {
	const loops = 4

	fake := &fakeAPI{}
	for i := range loops {
		fake.batches = append(fake.batches, []types.Message{message("user.created", `{"id":"1"}`, fmt.Sprintf("r-%d", i))})
	}

	sink := sqs.NewSink(fake, "https://queue.test/q")

	var handled atomic.Int32

	sink.Subscribe("user.created", func(_ context.Context, _ events.Envelope) error {
		handled.Add(1)

		return nil
	})

	var wg sync.WaitGroup

	for range loops {
		wg.Go(func() {
			if n, err := sink.ReceiveOnce(t.Context()); n != 1 || err != nil {
				t.Errorf("ReceiveOnce returned (%d, %v), want (1, nil)", n, err)
			}
		})
	}

	wg.Wait()

	if handled.Load() != loops || len(fake.deleted) != loops {
		t.Errorf("handled %d and deleted %v, want %d of each", handled.Load(), fake.deleted, loops)
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

// TestLogsThroughTheDefaultOfTheMoment: with no WithLogger the sink reads
// slog.Default() as it logs, not as it's built, so a host that calls
// slog.SetDefault after NewSink still hears it.
func TestLogsThroughTheDefaultOfTheMoment(t *testing.T) {
	fake := &fakeAPI{batches: [][]types.Message{{message("user.deleted", `{"id":"1"}`, "r-1")}}}
	sink := sqs.NewSink(fake, "https://queue.test/q")

	sink.Subscribe("user.deleted", func(_ context.Context, _ events.Envelope) error {
		return events.New("boom", nil)
	})

	// SetDefault rewires the log package too, so all three go back after.
	prev, out, flags := slog.Default(), log.Writer(), log.Flags()

	t.Cleanup(func() {
		slog.SetDefault(prev)
		log.SetOutput(out)
		log.SetFlags(flags)
	})

	var buf bytes.Buffer

	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	if _, err := sink.ReceiveOnce(t.Context()); err != nil {
		t.Fatalf("ReceiveOnce: %v", err)
	}

	if !strings.Contains(buf.String(), `msg="sqs: handler failed, leaving for redelivery"`) {
		t.Errorf("log = %q, want the nack on the default set after NewSink", buf.String())
	}
}
