package http

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
)

// Recoverer returns middleware that converts a handler panic into a 500
// "unknown" error instead of taking the connection down with nothing
// written. The value and stack go to slog, not the wire.
// http.ErrAbortHandler re-panics untouched, per net/http's contract.
// Register it first so it also covers everything registered after.
func Recoverer() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) (err error) {
			defer func() {
				p := recover()
				if p == nil {
					return
				}

				if p == http.ErrAbortHandler { //nolint:errorlint,goerr113 // identity is the contract for this sentinel
					panic(p)
				}

				slog.ErrorContext(r.Context(), "handler panic",
					slog.Any("panic", p),
					slog.String("stack", string(debug.Stack())),
				)

				err = New("unknown", nil, fmt.Errorf("panic: %v", p))
			}()

			return next(w, r)
		}
	}
}

// requestMetaKey carries the requestMeta attached by RequestMeta.
type requestMetaKey struct{}

type requestMeta struct {
	clientIP  string
	userAgent string
}

// RequestMeta returns middleware recording cross-cutting request metadata
// on the context, read back via ClientIP and UserAgent, so the minority of
// handlers wanting these don't carry them in every Req. trustForwarded lets
// X-Forwarded-For's leftmost entry beat RemoteAddr: the header is
// client-writable, so only set it behind a proxy that rewrites it, and
// treat the result as fit for logs and rate limits, never authorization.
func RequestMeta(trustForwarded bool) Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) error {
			ip := remoteIP(r)

			if trustForwarded {
				if fwd := forwardedClientIP(r); fwd != "" {
					ip = fwd
				}
			}

			meta := requestMeta{clientIP: ip, userAgent: r.UserAgent()}
			ctx := context.WithValue(r.Context(), requestMetaKey{}, meta)

			return next(w, r.WithContext(ctx))
		}
	}
}

// ClientIP returns the client IP RequestMeta recorded, or "" when the
// middleware isn't in the chain. See RequestMeta for how far to trust it.
func ClientIP(ctx context.Context) string {
	meta, _ := ctx.Value(requestMetaKey{}).(requestMeta)

	return meta.clientIP
}

// UserAgent returns the User-Agent RequestMeta recorded, or "" when the
// middleware isn't in the chain.
func UserAgent(ctx context.Context) string {
	meta, _ := ctx.Value(requestMetaKey{}).(requestMeta)

	return meta.userAgent
}

// remoteIP is RemoteAddr with the port stripped; RemoteAddr comes back
// verbatim when it doesn't parse as host:port.
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

// forwardedClientIP is X-Forwarded-For's leftmost entry: the original
// client as reported by whichever proxy appended last, which is why it's
// only worth reading behind a proxy you control.
func forwardedClientIP(r *http.Request) string {
	fwd := r.Header.Get("X-Forwarded-For")
	if fwd == "" {
		return ""
	}

	first, _, _ := strings.Cut(fwd, ",")

	return strings.TrimSpace(first)
}

// NotFoundHandler writes route_not_found in dfl's error shape. Unmatched
// routes are the mux's to handle, not the Router's, so wire it there:
// chi's NotFound hook, or a "/" fallback pattern on a ServeMux.
func NotFoundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		e := New("route_not_found", nil)
		writeJSON(w, e.StatusCode(), e)
	}
}

// MethodNotAllowedHandler is NotFoundHandler's sibling for muxes with a
// method-not-allowed hook, like chi: right path, wrong method, 405.
func MethodNotAllowedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		e := New("method_not_allowed", nil)
		writeJSON(w, e.StatusCode(), e)
	}
}
