// Package cher provides a Coercer that understands the cher errors from
// wearemojo/mojo-public-go and projects them into *dflhttp.ReqError. Opt-in:
// callers that don't use cher should use dflhttp.DefaultCoercer instead.
//
// It shares a name with the library it adapts, so alias it at use sites that
// need both:
//
//	import (
//		"github.com/wearemojo/mojo-public-go/lib/cher"
//
//		dflcher "github.com/duffleone/dfl/http/cher"
//	)
package cher

import (
	"errors"
	"maps"
	"strings"

	dflhttp "github.com/duffleone/dfl/http"
	mojocher "github.com/wearemojo/mojo-public-go/lib/cher"
)

// Coerce projects err into a *dflhttp.ReqError. Order of attempts:
//   - nil in, nil out
//   - existing *dflhttp.ReqError (via errors.As)
//   - cher.E: code and meta carry over, reasons carry over with their nesting
//     intact, and the status is the one cher assigns the code
//   - everything else: "unknown", which is a 500
//
// The last case deliberately does not go through mojocher.Coerce, which would
// put err.Error() in the meta of a 500. An error that reached here was never
// classified, so its text is as likely to be a driver message or a file path
// as something a client should read; it stays on the cause chain for logs
// instead, exactly as dflhttp.DefaultCoercer leaves it.
//
// cher.E's Extra field is dropped rather than projected. It holds whatever
// unrecognised JSON keys an upstream service's error body carried, kept for
// log forensics, and forwarding those to our own callers leaks one service's
// error shape through another.
func Coerce(err error) *dflhttp.ReqError {
	if err == nil {
		return nil
	}

	var reqErr *dflhttp.ReqError
	if errors.As(err, &reqErr) {
		return reqErr
	}

	var cherErr mojocher.E
	if errors.As(err, &cherErr) {
		return coerceCher(err, cherErr)
	}

	return dflhttp.Wrap(err, "unknown", nil)
}

// coerceCher does the projection. The status is set explicitly from cher's own
// StatusCode rather than left to ReqError's table. The two tables agree on
// most codes and both default to 400, but cher's is the contract a cher-shaped
// service already publishes, so it wins here and stays right if either drifts.
func coerceCher(err error, cherErr mojocher.E) *dflhttp.ReqError {
	code := strings.TrimSpace(cherErr.Code)
	if code == "" {
		code = mojocher.Unknown
	}

	// Derived from the effective code, not read off cherErr, so a blank code
	// lands on cher's 500 for Unknown rather than its 400 default.
	status := mojocher.E{Code: code}.StatusCode()

	out := dflhttp.Wrap(err, code, convertMeta(cherErr.Meta)).WithStatus(status)

	if reasons := convertReasons(cherErr.Reasons); len(reasons) > 0 {
		out = out.WithReasons(reasons...)
	}

	return out
}

// convertReasons walks cher's reason tree onto ReqError's. Both nest, so the
// shape survives: a reason keeps its own children rather than having them
// flattened up alongside it, and the client can still tell which failed check
// belongs to which.
func convertReasons(in []mojocher.E) []dflhttp.Reason {
	if len(in) == 0 {
		return nil
	}

	out := make([]dflhttp.Reason, 0, len(in))

	for _, reason := range in {
		out = append(out, dflhttp.Reason{
			Code:    reason.Code,
			Meta:    convertMeta(reason.Meta),
			Reasons: convertReasons(reason.Reasons),
		})
	}

	return out
}

// convertMeta copies m into a dflhttp.M. Both are map[string]any underneath,
// so a conversion would compile, but copying keeps the ReqError from sharing
// state with the cher error it came from: cher errors get passed around and
// re-wrapped, and a shared map would let one mutate the other's wire body.
// An empty map becomes nil so ReqError's `meta,omitempty` still drops the key.
func convertMeta(m mojocher.M) dflhttp.M {
	if len(m) == 0 {
		return nil
	}

	out := make(dflhttp.M, len(m))
	maps.Copy(out, m)

	return out
}
