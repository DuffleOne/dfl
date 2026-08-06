// Package cher provides a Coercer for the cher errors from
// wearemojo/mojo-public-go, projecting them into *dflhttp.ReqError.
// Opt-in: callers that don't use cher want dflhttp.DefaultCoercer. It
// shares a name with the library it adapts, so alias one of the two
// (dflcher, conventionally) at use sites importing both.
package cher

import (
	"errors"
	"maps"
	"strings"

	dflhttp "github.com/duffleone/dfl/http"
	mojocher "github.com/wearemojo/mojo-public-go/lib/cher"
)

// Coerce projects err into a *dflhttp.ReqError: nil passes through, an
// existing *ReqError wins, and a cher.E carries over its code, meta, and
// nested reasons on cher's own status table. Anything else is a 500
// "unknown" with the message left on the cause chain, not in the body:
// unclassified text is as likely a driver message as something a caller
// should read. cher.E's Extra is dropped, being upstream log forensics.
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
