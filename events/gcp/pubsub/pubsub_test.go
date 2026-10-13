package pubsub_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/pstest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/duffleone/dfl/events"
	gcppubsub "github.com/duffleone/dfl/events/gcp/pubsub"
)

// newClient returns a Pub/Sub client wired to an in-memory server that
// lives as long as the test.
func newClient(t *testing.T) *pubsub.Client {
	t.Helper()

	srv := pstest.NewServer()

	t.Cleanup(func() { _ = srv.Close() })

	conn, err := grpc.NewClient(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial the fake: %v", err)
	}

	client, err := pubsub.NewClient(t.Context(), "test-project", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("pubsub client: %v", err)
	}

	t.Cleanup(func() { _ = client.Close() })

	return client
}

// TestWithLoggerHearsAStoppedReceiver: a receiver that stops on an error,
// here a subscription the server has never heard of, says so on the
// configured logger with the event it was pulling.
func TestWithLoggerHearsAStoppedReceiver(t *testing.T) {
	var buf bytes.Buffer

	sink := gcppubsub.NewPullSink(newClient(t), "welcome-service",
		gcppubsub.WithLogger(slog.New(slog.NewTextHandler(&buf, nil))))

	sink.Subscribe("user.created", func(context.Context, events.Envelope) error { return nil })

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	if err := sink.Receive(ctx); err != nil {
		t.Fatalf("Receive = %v, want nil: the receiver should stop on its own, well inside the timeout", err)
	}

	for _, want := range []string{"level=ERROR", `msg="pubsub: receive stopped"`, "event=user.created"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("log = %q, want it to contain %q", buf.String(), want)
		}
	}
}
