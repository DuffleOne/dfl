package http

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// ReqError is the canonical http error type. Code, Meta, and Reasons go on
// the wire; the causes attached by New and Wrap stay internal, traversed
// by errors.Is and errors.As. There is no status in the body: it lives on
// the status line, derived from Code by StatusCode or forced by
// WithStatus, which keeps the wire shape identical to cher's.
type ReqError struct {
	Code    string   `json:"code"`
	Meta    M        `json:"meta,omitempty"`
	Reasons []Reason `json:"reasons,omitempty"`

	status int
	causes []error
}

// Reason is one machine-readable reason behind an error: each failed check
// becomes a Reason, so the client learns every problem in one round trip
// rather than fixing one per attempt. Code is the stable identifier to
// match on; Meta carries the detail, conventionally the field concerned
// and where it travelled ("in": "body"). Reasons nest, so a check that
// decomposes keeps its shape instead of flattening into one ambiguous list.
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

// New builds a ReqError. causes stay off the wire, recorded for errors.Is
// and errors.As; detail the client should see goes on with WithReasons.
// The status follows from code (see StatusCode), so most handlers never
// name one.
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

// StatusCode is the HTTP status to respond with: WithStatus's override
// when set, else the status Code maps to, else 400. The default is 400
// because a ReqError is an error somebody wrote down on purpose: part of
// the contract, not something that should page anyone. Codes that
// genuinely are the server's fault, like unknown, map to 500.
func (e *ReqError) StatusCode() int {
	if e.status != 0 {
		return e.status
	}

	if status, ok := codeStatuses[e.Code]; ok {
		return status
	}

	return http.StatusBadRequest
}

// WithStatus returns a copy of e that responds with status, whatever its
// Code would derive: a 409 on a conflicting write, a 502 from a dead
// upstream. The body is untouched, so callers still match on Code.
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
