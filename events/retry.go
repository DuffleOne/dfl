package events

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// notFoundCode is the EventError code IsNotFound recognises, so a handler that
// builds its own events.New("not_found", ...) with meta on it is retried just
// the same as one returning the NotFound sentinel.
const notFoundCode = "not_found"

// NotFound is the sentinel a handler returns when the event arrived before
// the state it describes: the row it names isn't in the database yet, and
// Retry treats that as worth waiting for, which is what makes emit-first
// safe. It's an *EventError so an unretried one still logs as not_found;
// return it, wrap it, or build your own with the same code and some meta.
// Named to sit beside pgxdb.NotFound, hence not ErrNotFound.
var NotFound = New(notFoundCode, nil) //nolint:errname // matches pgxdb.NotFound

// IsNotFound reports whether err means "the state this event describes isn't
// here yet": the NotFound sentinel anywhere on the chain, or an *EventError
// coded not_found. It's Retry's default Retryable.
func IsNotFound(err error) bool {
	if errors.Is(err, NotFound) {
		return true
	}

	var eventErr *EventError

	return errors.As(err, &eventErr) && eventErr.Code == notFoundCode
}

// ExhaustedFunc decides what happens to an event whose attempts ran out.
// Its return is what the delivery returns to the sink: nil acks the
// message, an error leaves it to the transport. Nack, Drop, and DeadLetter
// cover the usual answers; the type is exported so a service with its own
// idea (park in a table, page someone) can write one.
type ExhaustedFunc func(ctx context.Context, env Envelope, attempts int, err error) error

// Nack logs and hands the error back to the sink, leaving the decision to the
// transport. Retry's default, because a durable transport usually has its own
// answer already (an SQS redrive policy, a Pub/Sub dead-letter topic) and
// in-process retries shouldn't quietly overrule it.
var Nack ExhaustedFunc = func(ctx context.Context, env Envelope, attempts int, err error) error {
	logExhausted(ctx, env, attempts, "nack", err)

	return err
}

// Drop logs and acks. The event is gone: nothing redelivers it and nothing
// keeps a copy. Right when the event is advisory (a cache invalidation, a
// nudge to refresh something) and wrong when it carries the only record of
// work that still needs doing.
var Drop ExhaustedFunc = func(ctx context.Context, env Envelope, attempts int, err error) error {
	logExhausted(ctx, env, attempts, "drop", err)

	return nil
}

// DeadLetter logs, hands the envelope to park, and acks. park is where the
// event goes to be looked at or replayed later: an SQS DLQ, a dead-letter
// topic, a table. If park itself fails the error goes back to the sink,
// joined with the one that exhausted the retries: better redelivered than
// acked with no copy kept.
func DeadLetter(park func(ctx context.Context, env Envelope, err error) error) ExhaustedFunc {
	return func(ctx context.Context, env Envelope, attempts int, err error) error {
		logExhausted(ctx, env, attempts, "dead_letter", err)

		if parkErr := park(ctx, env, err); parkErr != nil {
			return errors.Join(err, parkErr)
		}

		return nil
	}
}

// Retry defaults, applied by RetryPolicy.withDefaults.
const (
	defaultAttempts    = 3
	defaultBackoffBase = 50 * time.Millisecond

	// maxBackoffDoublings caps ExponentialBackoff's shift so a policy with a
	// silly Attempts count can't overflow the duration.
	maxBackoffDoublings = 16
)

// RetryPolicy configures Retry. The zero value is usable: three attempts,
// exponential backoff from 50ms, retrying on not-found, handing the error back
// to the sink when they run out.
type RetryPolicy struct {
	// Attempts is the total number of tries including the first. Below 1
	// means the default.
	Attempts int

	// Backoff is how long to wait before attempt n+1, called with the number
	// of the attempt that just failed (1 for the first). nil means
	// ExponentialBackoff(50ms).
	Backoff func(attempt int) time.Duration

	// Retryable decides whether an error is worth another go. nil means
	// IsNotFound: an event that overtook its own write is the case worth
	// waiting on, and retrying a genuine bug just makes it happen three
	// times.
	Retryable func(error) bool

	// Exhausted runs when the attempts are spent. nil means Nack.
	Exhausted ExhaustedFunc
}

// ExponentialBackoff doubles base each time: base, 2×base, 4×base. Use it when
// the default 50ms is the wrong scale for how long your writes take to land.
func ExponentialBackoff(base time.Duration) func(attempt int) time.Duration {
	return func(attempt int) time.Duration {
		if attempt < 1 {
			attempt = 1
		}

		if attempt > maxBackoffDoublings {
			attempt = maxBackoffDoublings
		}

		return base << (attempt - 1)
	}
}

// FixedBackoff waits the same d every time.
func FixedBackoff(d time.Duration) func(attempt int) time.Duration {
	return func(_ int) time.Duration {
		return d
	}
}

// Retry is middleware that re-runs a handler when the event's subject
// isn't there yet, backing off between attempts, then applies the policy's
// Exhausted action. It makes emit-before-write survivable: a consumer that
// beats the committing transaction waits and looks again. Install with
// bus.Use, or on a single On. The waits hold the message's lease, so keep
// their sum well under the transport's visibility timeout or ack deadline.
func Retry(p RetryPolicy) Middleware {
	p = p.withDefaults()

	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, env Envelope) error {
			var err error

			for attempt := 1; attempt <= p.Attempts; attempt++ {
				err = next(ctx, env)
				if err == nil || !p.Retryable(err) {
					return err
				}

				if attempt == p.Attempts {
					break
				}

				// A cancelled context ends the retries early. The handler
				// error goes back with it, so the transport still sees a
				// failed delivery rather than a bare context error.
				if waitErr := wait(ctx, p.Backoff(attempt)); waitErr != nil {
					return errors.Join(err, waitErr)
				}
			}

			return p.Exhausted(ctx, env, p.Attempts, err)
		}
	}
}

func (p RetryPolicy) withDefaults() RetryPolicy {
	if p.Attempts < 1 {
		p.Attempts = defaultAttempts
	}

	if p.Backoff == nil {
		p.Backoff = ExponentialBackoff(defaultBackoffBase)
	}

	if p.Retryable == nil {
		p.Retryable = IsNotFound
	}

	if p.Exhausted == nil {
		p.Exhausted = Nack
	}

	return p
}

// wait sleeps for d, returning early if ctx ends. A non-positive d is a no-op,
// so a Backoff that returns 0 retries immediately.
func wait(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func logExhausted(ctx context.Context, env Envelope, attempts int, action string, err error) {
	slog.ErrorContext(ctx, "events: retries exhausted",
		slog.String("event", env.Name),
		slog.Int("attempts", attempts),
		slog.String("action", action),
		slog.String("error", err.Error()),
	)
}
