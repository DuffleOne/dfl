package sqs_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/duffleone/dfl/events"
	"github.com/duffleone/dfl/events/aws/sqs"
)

// fakeAPI is a scripted stand-in for the SQS client. SendMessage and
// DeleteMessage record their inputs; ReceiveMessage pops one batch per call
// and, once the script runs dry, blocks until ctx is cancelled, which is how
// a test tells the receive loop to stop.
type fakeAPI struct {
	sent    []*awssqs.SendMessageInput
	batches [][]types.Message
	deleted []string
}

func (f *fakeAPI) SendMessage(_ context.Context, in *awssqs.SendMessageInput, _ ...func(*awssqs.Options)) (*awssqs.SendMessageOutput, error) {
	f.sent = append(f.sent, in)

	return &awssqs.SendMessageOutput{}, nil
}

func (f *fakeAPI) ReceiveMessage(ctx context.Context, _ *awssqs.ReceiveMessageInput, _ ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error) {
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
