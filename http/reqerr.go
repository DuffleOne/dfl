package http

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// ReqError is the canonical http error type. Code, Meta, and Reasons are
// serialised on the wire; the causes attached by New and Wrap are internal,
// traversed by errors.Is and errors.As.
//
// The status is not part of the body. It's on the status line already, and
// leaving it off makes the shape the same as cher's, so a service can move
// between the two without its clients noticing. StatusCode derives it from
// Code instead; WithStatus overrides that for a code the table doesn't know.
type ReqError struct {
	Code    string   `json:"code"`
	Meta    M        `json:"meta,omitempty"`
	Reasons []Reason `json:"reasons,omitempty"`

	status int
	causes []error
}

// Reason is one machine-readable reason behind an error, for responses
// where a single code doesn't tell the caller what to fix: each failed
// check on a request becomes a Reason, so the client learns every problem
// in one round trip. Code is the stable identifier to match on; Meta
// carries the structured detail, conventionally the field it concerns and
// where that field travelled:
//
//	dflhttp.Reason{Code: "required", Meta: dflhttp.M{"in": "body", "field": "name"}}
//
// Reasons nest, so a check that decomposes keeps its shape: a rejected
// object field carries the failures of the fields inside it rather than
// flattening them all into one list where the client has to guess which
// belongs to which.
//
//	dflhttp.Reason{Code: "invalid", Meta: dflhttp.M{"in": "body", "field": "address"},
//	    Reasons: []dflhttp.Reason{
//	        {Code: "required", Meta: dflhttp.M{"field": "postcode"}},
//	    }}
type Reason struct {
	Code    string   `json:"code"`
	Meta    M        `json:"meta,omitempty"`
	Reasons []Reason `json:"reasons,omitempty"`
}

var _ error = (*ReqError)(nil)

// codeStatuses is the code-to-status table StatusCode consults. It holds the
// codes whose HTTP meaning is unambiguous: the canonical set cher uses, so
// the two agree on the wire, plus unsupported_media_type, which the request
// parser produces. Anything absent is a 400 (see StatusCode).
var codeStatuses = map[string]int{
	"bad_request":            http.StatusBadRequest,
	"unauthorized":           http.StatusUnauthorized,
	"access_denied":          http.StatusForbidden,
	"not_found":              http.StatusNotFound,
	"route_not_found":        http.StatusNotFound,
	"method_not_allowed":     http.StatusMethodNotAllowed,
	"endpoint_withdrawn":     http.StatusGone,
	"unsupported_media_type": http.StatusUnsupportedMediaType,
	"too_many_requests":      http.StatusTooManyRequests,
	"unknown":                http.StatusInternalServerError,
}

// New builds a ReqError. causes (if any) are recorded for errors.Is and
// errors.As traversal and stay off the wire; detail the client should see
// goes on with WithReasons.
//
// The status follows from code (see StatusCode), so most handlers never name
// one. Reach for WithStatus when yours needs a status the table doesn't
// derive.
func New(code string, meta M, causes ...error) *ReqError {
	return &ReqError{
		Code:   code,
		Meta:   meta,
		causes: causes,
	}
}

// Wrap builds a ReqError that wraps err as its primary cause. Additional
// causes are recorded after.
func Wrap(err error, code string, meta M, causes ...error) *ReqError {
	all := make([]error, 0, 1+len(causes))
	all = append(all, err)
	all = append(all, causes...)

	return &ReqError{
		Code:   code,
		Meta:   meta,
		causes: all,
	}
}

// StatusCode is the HTTP status to respond with: the one WithStatus set if
// there is one, otherwise the one Code maps to, otherwise 400.
//
// 400 is the default because a ReqError is an error somebody wrote down on
// purpose. That makes it part of the contract, something the caller can act
// on, and not the sort of thing that should page anyone. Codes that genuinely
// are the server's fault say so: "unknown", which is what DefaultCoercer
// gives an error nothing classified, maps to 500.
func (e *ReqError) StatusCode() int {
	if e.status != 0 {
		return e.status
	}

	if status, ok := codeStatuses[e.Code]; ok {
		return status
	}

	return http.StatusBadRequest
}

// WithStatus returns a copy of e that responds with status, whatever its Code
// would otherwise derive. For the statuses outside the small canonical set:
// a 409 on a conflicting write, a 402, a 502 from a dead upstream.
//
// It changes the status line only. Nothing about the body changes, so the
// caller still matches on Code.
func (e *ReqError) WithStatus(status int) *ReqError {
	out := *e
	out.status = status

	return &out
}

// WithReasons returns a copy of e with reasons appended to its wire
// Reasons. Copying keeps shared ReqError values safe: a package-level
// sentinel can be decorated per request without data races or reasons
// leaking between requests.
func (e *ReqError) WithReasons(reasons ...Reason) *ReqError {
	out := *e
	out.Reasons = append(slices.Clip(e.Reasons), reasons...)

	return &out
}

func (e *ReqError) Error() string {
	return fmt.Sprintf("%s keys=%s", e.Code, strings.Join(e.Meta.Keys(), ", "))
}

// Unwrap returns every cause, so errors.Is and errors.As traverse the
// whole set, not just the first. Note this is the multi-error form:
// errors.Unwrap, which only knows the single-error one, returns nil on a
// ReqError.
func (e *ReqError) Unwrap() []error {
	return e.causes
}

// DefaultCoercer is the minimal Coercer: it returns *ReqError as-is (via
// errors.As) and otherwise wraps err as "unknown", which StatusCode maps to
// 500. It does not know about samber/oops, cher, or any other third-party
// error type. For those see github.com/duffleone/dfl/http/oops and
// github.com/duffleone/dfl/http/cher.
func DefaultCoercer(err error) *ReqError {
	if err == nil {
		return nil
	}

	var reqErr *ReqError
	if errors.As(err, &reqErr) {
		return reqErr
	}

	return Wrap(err, "unknown", nil)
}
