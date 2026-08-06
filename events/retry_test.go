package events_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/duffleone/dfl/events"
)

// noBackoff keeps the tests instant. Retry's own timing is covered by
// TestExponentialBackoff and TestRetryHonoursBackoff.
func noBackoff(int) time.Duration { return 0 }

// countingHandler returns a HandlerFunc that fails with err for the first n
// calls and succeeds after, along with a pointer to the call count.
func countingHandler(failures int, err error) (events.HandlerFunc, *atomic.Int64) {
	var calls atomic.Int64

	return func(context.Context, events.Envelope) error {
		if calls.Add(1) <= int64(failures) {
			return err
		}

		return nil
	}, &calls
}

// TestIsNotFound covers both routes into Retry's default Retryable: the
// sentinel and the not_found EventError code.
func TestIsNotFound(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"the sentinel itself", events.NotFound, true},
		{"wrapped sentinel", fmt.Errorf("loading user: %w", events.NotFound), true},
		{"EventError coded not_found", events.New("not_found", nil), true},
		{"EventError wrapping the sentinel", events.Wrap(events.NotFound, "lookup_failed", nil), true},
		{"EventError with another code", events.New("validation_failed", nil), false},
		{"unrelated error", errors.New("boom"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := events.IsNotFound(tc.err); got != tc.want {
				t.Errorf("IsNotFound(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestRetrySucceedsOnLaterAttempt is the case the whole thing exists for: the
// event overtook its own write, and looking again a moment later finds it.
func TestRetrySucceedsOnLaterAttempt(t *testing.T) {
	handler, calls := countingHandler(2, events.NotFound)

	mw := events.Retry(events.RetryPolicy{Attempts: 3, Backoff: noBackoff})

	if err := mw(handler)(t.Context(), events.Envelope{Name: "user.created"}); err != nil {
		t.Fatalf("delivery = %v, want nil once the row shows up", err)
	}

	if got := calls.Load(); got != 3 {
		t.Errorf("handler called %d times, want 3", got)
	}
}

// TestRetryStopsOnFirstSuccess: a handler that works first time is called once.
func TestRetryStopsOnFirstSuccess(t *testing.T) {
	handler, calls := countingHandler(0, events.NotFound)

	mw := events.Retry(events.RetryPolicy{Backoff: noBackoff})

	if err := mw(handler)(t.Context(), events.Envelope{}); err != nil {
		t.Fatalf("delivery = %v, want nil", err)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("handler called %d times, want 1", got)
	}
}

// TestRetryIgnoresOtherErrors: only not-found is worth waiting on. A genuine
// bug should surface once, not three times.
func TestRetryIgnoresOtherErrors(t *testing.T) {
	boom := errors.New("boom")
	handler, calls := countingHandler(99, boom)

	mw := events.Retry(events.RetryPolicy{Backoff: noBackoff})

	if err := mw(handler)(t.Context(), events.Envelope{}); !errors.Is(err, boom) {
		t.Fatalf("delivery = %v, want the handler error back", err)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("handler called %d times, want 1", got)
	}
}

// TestRetryDefaultAttempts pins the documented default of three.
func TestRetryDefaultAttempts(t *testing.T) {
	handler, calls := countingHandler(99, events.NotFound)

	mw := events.Retry(events.RetryPolicy{Backoff: noBackoff})
	_ = mw(handler)(t.Context(), events.Envelope{})

	if got := calls.Load(); got != 3 {
		t.Errorf("handler called %d times, want the default 3", got)
	}
}

// TestRetryExhaustedNack is the default: the error goes back to the sink so
// the transport's own redrive policy decides.
func TestRetryExhaustedNack(t *testing.T) {
	handler, _ := countingHandler(99, events.NotFound)

	mw := events.Retry(events.RetryPolicy{Attempts: 2, Backoff: noBackoff})

	if err := mw(handler)(t.Context(), events.Envelope{}); !errors.Is(err, events.NotFound) {
		t.Errorf("delivery = %v, want the not-found error handed back", err)
	}
}

// TestRetryExhaustedDrop: the event is acked away, so the sink sees success
// and nothing redelivers.
func TestRetryExhaustedDrop(t *testing.T) {
	handler, calls := countingHandler(99, events.NotFound)

	mw := events.Retry(events.RetryPolicy{
		Attempts:  2,
		Backoff:   noBackoff,
		Exhausted: events.Drop,
	})

	if err := mw(handler)(t.Context(), events.Envelope{}); err != nil {
		t.Errorf("delivery = %v, want nil so the transport acks", err)
	}

	if got := calls.Load(); got != 2 {
		t.Errorf("handler called %d times, want 2", got)
	}
}

// TestRetryExhaustedDeadLetter: the envelope is parked, then acked.
func TestRetryExhaustedDeadLetter(t *testing.T) {
	var parked []events.Envelope

	var parkedErr error

	handler, _ := countingHandler(99, events.NotFound)

	mw := events.Retry(events.RetryPolicy{
		Attempts: 2,
		Backoff:  noBackoff,
		Exhausted: events.DeadLetter(func(_ context.Context, env events.Envelope, err error) error {
			parked = append(parked, env)
			parkedErr = err

			return nil
		}),
	})

	env := events.Envelope{Name: "user.created", Payload: []byte(`{"id":"1"}`)}

	if err := mw(handler)(t.Context(), env); err != nil {
		t.Errorf("delivery = %v, want nil once parked", err)
	}

	if len(parked) != 1 || parked[0].Name != "user.created" {
		t.Fatalf("parked = %v, want the one envelope", parked)
	}

	if !errors.Is(parkedErr, events.NotFound) {
		t.Errorf("parked error = %v, want the not-found error", parkedErr)
	}
}

// TestRetryDeadLetterFailureNacks: if parking fails the event isn't acked, or
// it would be gone with nothing keeping a copy.
func TestRetryDeadLetterFailureNacks(t *testing.T) {
	dlqDown := errors.New("dlq unreachable")
	handler, _ := countingHandler(99, events.NotFound)

	mw := events.Retry(events.RetryPolicy{
		Attempts: 2,
		Backoff:  noBackoff,
		Exhausted: events.DeadLetter(func(context.Context, events.Envelope, error) error {
			return dlqDown
		}),
	})

	err := mw(handler)(t.Context(), events.Envelope{})

	if !errors.Is(err, dlqDown) {
		t.Errorf("delivery = %v, want the parking failure back", err)
	}

	if !errors.Is(err, events.NotFound) {
		t.Errorf("delivery = %v, want the original not-found joined in too", err)
	}
}

// TestRetryCustomRetryable: the predicate is swappable, for codebases whose
// not-found doesn't go through events.NotFound (pgxdb.NotFound, say).
func TestRetryCustomRetryable(t *testing.T) {
	tooEarly := errors.New("too early")
	handler, calls := countingHandler(1, tooEarly)

	mw := events.Retry(events.RetryPolicy{
		Attempts:  3,
		Backoff:   noBackoff,
		Retryable: func(err error) bool { return errors.Is(err, tooEarly) },
	})

	if err := mw(handler)(t.Context(), events.Envelope{}); err != nil {
		t.Fatalf("delivery = %v, want nil", err)
	}

	if got := calls.Load(); got != 2 {
		t.Errorf("handler called %d times, want 2", got)
	}
}

// TestRetryStopsOnContextCancellation: the wait is interruptible, and the
// handler error still goes back so the delivery reads as failed.
func TestRetryStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	handler, calls := countingHandler(99, events.NotFound)

	mw := events.Retry(events.RetryPolicy{
		Attempts: 5,
		Backoff:  events.FixedBackoff(time.Hour),
	})

	err := mw(handler)(ctx, events.Envelope{})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("delivery = %v, want the cancellation", err)
	}

	if !errors.Is(err, events.NotFound) {
		t.Errorf("delivery = %v, want the handler error joined in", err)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("handler called %d times, want 1 before the cancelled wait", got)
	}
}

// TestRetryHonoursBackoff checks the wait actually happens between attempts.
func TestRetryHonoursBackoff(t *testing.T) {
	handler, _ := countingHandler(2, events.NotFound)

	mw := events.Retry(events.RetryPolicy{
		Attempts: 3,
		Backoff:  events.FixedBackoff(20 * time.Millisecond),
	})

	start := time.Now()

	if err := mw(handler)(t.Context(), events.Envelope{}); err != nil {
		t.Fatalf("delivery = %v, want nil", err)
	}

	// Two waits between three attempts.
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Errorf("took %s, want at least 40ms of backoff", elapsed)
	}
}

// TestExponentialBackoff pins the doubling and the guards at both ends.
func TestExponentialBackoff(t *testing.T) {
	backoff := events.ExponentialBackoff(10 * time.Millisecond)

	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 10 * time.Millisecond},
		{1, 10 * time.Millisecond},
		{2, 20 * time.Millisecond},
		{3, 40 * time.Millisecond},
		{4, 80 * time.Millisecond},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.attempt), func(t *testing.T) {
			if got := backoff(tc.attempt); got != tc.want {
				t.Errorf("backoff(%d) = %s, want %s", tc.attempt, got, tc.want)
			}
		})
	}

	// Capped, not overflowed.
	if got := backoff(1000); got <= 0 {
		t.Errorf("backoff(1000) = %s, want a positive capped duration", got)
	}
}

// TestRetryThroughTheBus wires the middleware where it actually goes, checking
// a handler that finds nothing on the first delivery still does its work.
func TestRetryThroughTheBus(t *testing.T) {
	var (
		created atomic.Bool
		handled = make(chan struct{})
	)

	bus := events.NewBus(events.NewMemSink())
	bus.Use(events.Retry(events.RetryPolicy{
		Attempts: 5,
		Backoff:  events.FixedBackoff(5 * time.Millisecond),
	}))

	bus.On(func(context.Context, testUserCreated) error {
		if !created.Load() {
			return events.New("not_found", events.M{"id": "1"})
		}

		close(handled)

		return nil
	})

	if err := bus.Emit(t.Context(), testUserCreated{ID: "1"}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	// The write lands after the event is already in flight, which is the
	// ordering the retry is there to absorb.
	time.Sleep(10 * time.Millisecond)
	created.Store(true)

	select {
	case <-handled:
	case <-time.After(2 * time.Second):
		t.Fatal("handler never succeeded after the row appeared")
	}
}

type testUserCreated struct {
	ID string `json:"id"`
}

func (testUserCreated) EventName() string { return "test.user.created" }
