package http

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// ReqError is the canonical http error type. Code, StatusCode, Meta, and
// Reasons are serialised on the wire; the causes attached by New and Wrap
// are internal, traversed by errors.Is and errors.As.
//
// StatusCode is omitted from the body when zero, so a contract that keeps
// the status out of the body (it's on the status line already) can have
// its Coercer or ErrorWriter zero the field on a copy before encoding.
type ReqError struct {
	Code       string   `json:"code"`
	StatusCode int      `json:"status_code,omitzero"`
	Meta       M        `json:"meta,omitempty"`
	Reasons    []Reason `json:"reasons,omitempty"`

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
type Reason struct {
	Code string `json:"code"`
	Meta M      `json:"meta,omitempty"`
}

var _ error = (*ReqError)(nil)

// New builds a ReqError. causes (if any) are recorded for errors.Is and
// errors.As traversal and stay off the wire; detail the client should see
// goes on with WithReasons.
func New(statusCode int, code string, meta M, causes ...error) *ReqError {
	return &ReqError{
		StatusCode: statusCode,
		Code:       code,
		Meta:       meta,
		causes:     causes,
	}
}

// Wrap builds a ReqError that wraps err as its primary cause. Additional
// causes are recorded after.
func Wrap(err error, statusCode int, code string, meta M, causes ...error) *ReqError {
	all := make([]error, 0, 1+len(causes))
	all = append(all, err)
	all = append(all, causes...)

	return &ReqError{
		StatusCode: statusCode,
		Code:       code,
		Meta:       meta,
		causes:     all,
	}
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
// errors.As) and otherwise wraps err as 500 "unknown". It does not know
// about samber/oops or any other third-party error type. For oops support
// see github.com/duffleone/dfl/http/oops.
func DefaultCoercer(err error) *ReqError {
	if err == nil {
		return nil
	}

	var reqErr *ReqError
	if errors.As(err, &reqErr) {
		return reqErr
	}

	return Wrap(err, http.StatusInternalServerError, "unknown", nil)
}
