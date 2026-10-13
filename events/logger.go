package events

import (
	"context"
	"log/slog"
)

// loggerKey carries the bus's logger on a delivery's context.
type loggerKey struct{}

// WithLogger sets where the bus logs: the default ErrorHandler, and Retry
// when its attempts run out. Unset, that's slog.Default() as of each log,
// so a host that calls slog.SetDefault after building the bus is still
// honoured.
func WithLogger(l *slog.Logger) Option {
	return func(b *Bus) {
		b.logger = l
	}
}

// withLogger puts the bus's logger on a delivery's ctx, which is how the
// ErrorHandler and middleware such as Retry come by it.
func (b *Bus) withLogger(ctx context.Context) context.Context {
	if b.logger == nil {
		return ctx
	}

	return context.WithValue(ctx, loggerKey{}, b.logger)
}

// loggerFrom returns the logger the bus put on ctx, or the process default
// as of now when there isn't one.
func loggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return l
	}

	return slog.Default()
}
