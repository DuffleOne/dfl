package http

import (
	"context"
	"log/slog"
)

// loggerKey carries the router's logger on a request's context.
type loggerKey struct{}

// WithLogger sets where the router's own middleware logs, which today is
// Recoverer and its panics. Unset, that's slog.Default() as of each log, so
// a host that calls slog.SetDefault after building the router is still
// honoured.
func WithLogger(l *slog.Logger) Option {
	return func(r *Router) {
		r.logger = l
	}
}

// loggerFrom returns the logger the router put on ctx, or the process
// default as of now when there isn't one.
func loggerFrom(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok {
		return l
	}

	return slog.Default()
}
