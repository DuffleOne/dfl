// Package bustest exports a conformance suite that every events.Sink backend
// should pass. Each backend's *_test.go calls Run with a Factory that builds a
// Bus over that backend; the suite then exercises the transport-agnostic
// behaviour we care about (delivery, fan-out, validation on both sides, error
// routing, middleware composition).
//
// Behaviour that's specific to one backend (synchronous vs async delivery,
// ordering, durability) is deliberately not asserted here; it lives in that
// backend's own tests. The MemSink suite caller is events/mem_test.go.
package bustest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/duffleone/dfl/events"
)

// Factory tells the suite how to build a fresh bus and the sink behind it. The
// sink is returned too so the suite can inject raw envelopes to exercise the
// consume side (decode and deliver-time validation) without going through Emit.
type Factory struct {
	New func(opts ...events.Option) (*events.Bus, events.Sink)
}

// Run executes the full conformance suite against f.
func Run(t *testing.T, f Factory) {
	t.Helper()

	t.Run("DeliversToSubscriber", func(t *testing.T) { deliversToSubscriber(t, f) })
	t.Run("FanOutRunsEvery", func(t *testing.T) { fanOutRunsEvery(t, f) })
	t.Run("EmitReturnsNilOnCleanPublish", func(t *testing.T) { emitReturnsNilOnCleanPublish(t, f) })
	t.Run("UnknownEventIsNoop", func(t *testing.T) { unknownEventIsNoop(t, f) })
	t.Run("ValidationRejectsAtEmit", func(t *testing.T) { validationRejectsAtEmit(t, f) })
	t.Run("ValidationRejectsAtDeliver", func(t *testing.T) { validationRejectsAtDeliver(t, f) })
	t.Run("DecodeFailureReachesErrorHandler", func(t *testing.T) { decodeFailureReachesErrorHandler(t, f) })
	t.Run("HandlerErrorReachesErrorHandler", func(t *testing.T) { handlerErrorReachesErrorHandler(t, f) })
	t.Run("CustomValidatorHonoured", func(t *testing.T) { customValidatorHonoured(t, f) })
	t.Run("CustomCoercerHonoured", func(t *testing.T) { customCoercerHonoured(t, f) })
	t.Run("MiddlewareComposesInOrder", func(t *testing.T) { middlewareComposesInOrder(t, f) })
	t.Run("MiddlewareShortCircuits", func(t *testing.T) { middlewareShortCircuits(t, f) })
}

// --- sample events ---

type ping struct {
	Seq int `json:"seq"`
}

func (ping) EventName() string { return "bustest.ping" }

type needsEmail struct {
	Email string `json:"email"`
}

func (needsEmail) EventName() string { return "bustest.needs_email" }

func (e needsEmail) Validate() error {
	if e.Email == "" {
		return events.New("validation_failed", events.M{"field": "email"})
	}

	return nil
}

// --- helpers ---

const wait = time.Second

func recv[T any](t *testing.T, ch <-chan T) T {
	t.Helper()

	select {
	case v := <-ch:
		return v
	case <-time.After(wait):
		t.Fatal("timed out waiting for delivery")

		var zero T

		return zero
	}
}

func notReceived[T any](t *testing.T, ch <-chan T) {
	t.Helper()

	select {
	case <-ch:
		t.Fatal("handler ran when it should not have")
	default:
	}
}

// --- subtests ---

func deliversToSubscriber(t *testing.T, f Factory) {
	bus, _ := f.New()

	got := make(chan ping, 1)
	bus.On(func(_ context.Context, e ping) error {
		got <- e

		return nil
	})

	if err := bus.Emit(context.Background(), ping{Seq: 7}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if e := recv(t, got); e.Seq != 7 {
		t.Errorf("seq = %d, want 7", e.Seq)
	}
}

func fanOutRunsEvery(t *testing.T, f Factory) {
	bus, _ := f.New()

	a := make(chan struct{}, 1)
	b := make(chan struct{}, 1)
	bus.On(func(_ context.Context, _ ping) error { a <- struct{}{}; return nil })
	bus.On(func(_ context.Context, _ ping) error { b <- struct{}{}; return nil })

	if err := bus.Emit(context.Background(), ping{Seq: 1}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	recv(t, a)
	recv(t, b)
}

func emitReturnsNilOnCleanPublish(t *testing.T, f Factory) {
	bus, _ := f.New()

	done := make(chan struct{}, 1)
	bus.On(func(_ context.Context, _ ping) error { done <- struct{}{}; return nil })

	if err := bus.Emit(context.Background(), ping{Seq: 1}); err != nil {
		t.Errorf("emit returned %v, want nil", err)
	}

	recv(t, done)
}

func unknownEventIsNoop(t *testing.T, f Factory) {
	bus, _ := f.New()

	if err := bus.Emit(context.Background(), ping{Seq: 1}); err != nil {
		t.Errorf("emit with no subscribers returned %v, want nil", err)
	}
}

func validationRejectsAtEmit(t *testing.T, f Factory) {
	bus, _ := f.New()

	ran := make(chan struct{}, 1)
	bus.On(func(_ context.Context, _ needsEmail) error { ran <- struct{}{}; return nil })

	err := bus.Emit(context.Background(), needsEmail{Email: ""})
	if err == nil {
		t.Fatal("emit of invalid event returned nil, want error")
	}

	var eventErr *events.EventError
	if !errors.As(err, &eventErr) || eventErr.Code != "validation_failed" {
		t.Fatalf("emit error = %v, want code validation_failed", err)
	}

	// Invalid event must not have been published, so no handler ran.
	notReceived(t, ran)
}

func validationRejectsAtDeliver(t *testing.T, f Factory) {
	errs := make(chan *events.EventError, 1)
	bus, sink := f.New(events.WithErrorHandler(func(_ context.Context, _ events.Envelope, err *events.EventError) {
		errs <- err
	}))

	ran := make(chan struct{}, 1)
	bus.On(func(_ context.Context, _ needsEmail) error { ran <- struct{}{}; return nil })

	// Inject a raw envelope that bypasses Emit's producer-side validation, as a
	// different process publishing over a real transport would.
	_ = sink.Publish(context.Background(), events.Envelope{
		Name:    needsEmail{}.EventName(),
		Payload: []byte(`{"email":""}`),
	})

	if got := recv(t, errs); got.Code != "validation_failed" {
		t.Errorf("error code = %q, want validation_failed", got.Code)
	}

	notReceived(t, ran)
}

func decodeFailureReachesErrorHandler(t *testing.T, f Factory) {
	errs := make(chan *events.EventError, 1)
	bus, sink := f.New(events.WithErrorHandler(func(_ context.Context, _ events.Envelope, err *events.EventError) {
		errs <- err
	}))

	ran := make(chan struct{}, 1)
	bus.On(func(_ context.Context, _ ping) error { ran <- struct{}{}; return nil })

	_ = sink.Publish(context.Background(), events.Envelope{
		Name:    ping{}.EventName(),
		Payload: []byte(`{not json`),
	})

	if got := recv(t, errs); got.Code != "decode_failed" {
		t.Errorf("error code = %q, want decode_failed", got.Code)
	}

	notReceived(t, ran)
}

func handlerErrorReachesErrorHandler(t *testing.T, f Factory) {
	errs := make(chan *events.EventError, 1)
	bus, _ := f.New(events.WithErrorHandler(func(_ context.Context, _ events.Envelope, err *events.EventError) {
		errs <- err
	}))

	bus.On(func(_ context.Context, _ ping) error {
		return events.New("boom", events.M{"why": "test"})
	})

	if err := bus.Emit(context.Background(), ping{Seq: 1}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	got := recv(t, errs)
	if got.Code != "boom" {
		t.Errorf("error code = %q, want boom", got.Code)
	}

	wantName := ping{}.EventName()
	if got.Event != wantName {
		t.Errorf("error event = %q, want %q", got.Event, wantName)
	}
}

func customValidatorHonoured(t *testing.T, f Factory) {
	bus, _ := f.New(events.WithValidator(rejectAll{}))

	if err := bus.Emit(context.Background(), ping{Seq: 1}); err == nil {
		t.Fatal("emit returned nil, want the custom validator's error")
	}
}

type rejectAll struct{}

func (rejectAll) Validate(_ events.Event) error {
	return events.New("rejected", nil)
}

func customCoercerHonoured(t *testing.T, f Factory) {
	errs := make(chan *events.EventError, 1)
	bus, _ := f.New(
		events.WithCoercer(func(error) *events.EventError { return events.New("coerced", nil) }),
		events.WithErrorHandler(func(_ context.Context, _ events.Envelope, err *events.EventError) { errs <- err }),
	)

	bus.On(func(_ context.Context, _ ping) error { return errSentinel })

	if err := bus.Emit(context.Background(), ping{Seq: 1}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if got := recv(t, errs); got.Code != "coerced" {
		t.Errorf("error code = %q, want coerced", got.Code)
	}
}

var errSentinel = events.New("sentinel", nil)

func middlewareComposesInOrder(t *testing.T, f Factory) {
	bus, _ := f.New()

	order := make(chan string, 8)
	tap := func(label string) events.Middleware {
		return func(next events.HandlerFunc) events.HandlerFunc {
			return func(ctx context.Context, env events.Envelope) error {
				order <- label

				return next(ctx, env)
			}
		}
	}

	bus.Use(tap("group"))

	done := make(chan struct{}, 1)
	bus.On(func(_ context.Context, _ ping) error {
		order <- "handler"
		done <- struct{}{}

		return nil
	}, tap("route"))

	if err := bus.Emit(context.Background(), ping{Seq: 1}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	recv(t, done)

	want := []string{"group", "route", "handler"}
	for _, w := range want {
		if got := recv(t, order); got != w {
			t.Errorf("order: got %q, want %q", got, w)
		}
	}
}

func middlewareShortCircuits(t *testing.T, f Factory) {
	errs := make(chan *events.EventError, 1)
	bus, _ := f.New(events.WithErrorHandler(func(_ context.Context, _ events.Envelope, err *events.EventError) {
		errs <- err
	}))

	ran := make(chan struct{}, 1)
	stop := func(events.HandlerFunc) events.HandlerFunc {
		return func(_ context.Context, _ events.Envelope) error {
			return events.New("blocked", nil)
		}
	}

	bus.On(func(_ context.Context, _ ping) error { ran <- struct{}{}; return nil }, stop)

	if err := bus.Emit(context.Background(), ping{Seq: 1}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if got := recv(t, errs); got.Code != "blocked" {
		t.Errorf("error code = %q, want blocked", got.Code)
	}

	notReceived(t, ran)
}
