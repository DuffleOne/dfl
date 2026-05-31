// Package otel is an events.Plugin that propagates OpenTelemetry trace context
// through events and records a span on each publish and delivery.
//
// Install it on a bus with events.WithPlugins. On publish it starts a producer
// span and injects the trace context into the envelope headers; on deliver it
// extracts that context and starts a consumer span as its child, so a single
// trace flows from the emitter, across the transport, into the handler:
//
//	bus := events.NewBus(sink, events.WithPlugins(otel.New()))
//
// Cross-process propagation needs the sink to carry Envelope.Headers over its
// transport. MemSink does, and the cloud adapters map them to native message
// attributes.
package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/duffleone/dfl/events"
)

// scopeName is the instrumentation scope for spans this plugin creates.
const scopeName = "github.com/duffleone/dfl/events"

// Plugin propagates trace context and records spans. Build one with New.
type Plugin struct {
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

var _ events.Plugin = (*Plugin)(nil)

// Option configures a Plugin.
type Option func(*Plugin)

// WithTracerProvider sets the TracerProvider spans are created from. Defaults to
// the global provider (otel.GetTracerProvider).
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(p *Plugin) { p.tracer = tp.Tracer(scopeName) }
}

// WithPropagator sets the propagator used to inject and extract trace context.
// Defaults to the global propagator (otel.GetTextMapPropagator), usually the
// W3C trace-context propagator.
func WithPropagator(prop propagation.TextMapPropagator) Option {
	return func(p *Plugin) { p.propagator = prop }
}

// New builds the plugin. By default it uses the globally registered
// TracerProvider and TextMapPropagator, so it picks up whatever the app set up
// with otel.SetTracerProvider and otel.SetTextMapPropagator.
func New(opts ...Option) *Plugin {
	p := &Plugin{
		tracer:     otel.Tracer(scopeName),
		propagator: otel.GetTextMapPropagator(),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// WrapPublish starts a producer span and injects the trace context into the
// envelope headers before the event is sent.
func (p *Plugin) WrapPublish(next events.PublishFunc) events.PublishFunc {
	return func(ctx context.Context, env *events.Envelope) error {
		ctx, span := p.tracer.Start(ctx, env.Name+" publish",
			trace.WithSpanKind(trace.SpanKindProducer),
		)
		defer span.End()

		if env.Headers == nil {
			env.Headers = map[string]string{}
		}

		p.propagator.Inject(ctx, propagation.MapCarrier(env.Headers))

		if err := next(ctx, env); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return err
		}

		return nil
	}
}

// WrapDeliver extracts the trace context from the envelope headers and starts a
// consumer span as its child around the handler.
func (p *Plugin) WrapDeliver(next events.HandlerFunc) events.HandlerFunc {
	return func(ctx context.Context, env events.Envelope) error {
		ctx = p.propagator.Extract(ctx, propagation.MapCarrier(env.Headers))

		ctx, span := p.tracer.Start(ctx, env.Name+" process",
			trace.WithSpanKind(trace.SpanKindConsumer),
		)
		defer span.End()

		if err := next(ctx, env); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())

			return err
		}

		return nil
	}
}
