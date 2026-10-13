// Command poller drives the SQS sink from a host's own loop: one ReceiveOnce
// a turn, the host choosing what to report and how long to wait on a failure.
//
// Run (needs AWS credentials and a queue), Ctrl-C to stop:
//
//	QUEUE_URL=https://sqs.eu-west-1.amazonaws.com/123/events go run .
package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/duffleone/dfl/events"
	"github.com/duffleone/dfl/events/aws/sqs"
)

// UserCreated is the example event.
type UserCreated struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (UserCreated) EventName() string { return "user.created" }

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Ctrl-C ends ctx. The turn in flight finishes the message it's on,
	// deletes it, and leaves the rest of the batch on the queue.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	sink := sqs.NewSink(awssqs.NewFromConfig(cfg), os.Getenv("QUEUE_URL"),
		sqs.WithLongPoll(5), // short turns, so an idle queue still reports often
		sqs.WithBatchSize(5),
		sqs.WithLogger(logger.With(slog.String("component", "sqs"))),
	)
	bus := events.NewBus(sink)

	bus.On(func(ctx context.Context, e UserCreated) error {
		logger.InfoContext(ctx, "welcome", slog.String("email", e.Email))

		return nil
	})

	if err := bus.Emit(ctx, UserCreated{ID: "1", Email: "a@b.com"}); err != nil {
		return err
	}

	for ctx.Err() == nil {
		started := time.Now()

		n, err := sink.ReceiveOnce(ctx)

		switch {
		case err == nil:
			// Where a real host tells its health check the turn went fine.
			logger.InfoContext(ctx, "turn", slog.Int("messages", n), slog.Duration("took", time.Since(started)))
		case ctx.Err() != nil:
			logger.InfoContext(ctx, "stopping", slog.Int("handled_this_turn", n))
		default:
			// ReceiveOnce never waits after a failed poll: the pause is ours.
			logger.ErrorContext(ctx, "turn failed", slog.String("error", err.Error()))
			pause(ctx, 5*time.Second)
		}
	}

	return nil
}

// pause waits for d, or less if ctx ends first.
func pause(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
