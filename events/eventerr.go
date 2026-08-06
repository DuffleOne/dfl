package events

import (
	"errors"
	"fmt"
	"strings"
)

// EventError is the canonical events error type, the bus analog of the http
// package's ReqError. Code, Event, and Meta are serialised on the wire; the
// causes attached by New and Wrap are internal, traversed by errors.Is and
// errors.As.
//
// There's no status here: a bus has no HTTP status to carry. The Event field
// names the event the error relates to and is stamped by the bus when it has
// the name. RegisterEndpoint decides the status from Code when it projects an
// EventError back onto a ReqError.
type EventError struct {
	Code  string `json:"code"`
	Event string `json:"event,omitempty"`
	Meta  M      `json:"meta,omitempty"`

	causes []error
}

var _ error = (*EventError)(nil)

// New builds an EventError. causes (if any) are recorded for errors.Is and
// errors.As traversal and stay off the wire.
func New(code string, meta M, causes ...error) *EventError {
	return &EventError{
		Code:   code,
		Meta:   meta,
		causes: causes,
	}
}

// Wrap builds an EventError that wraps err as its primary cause. Additional
// causes are recorded after.
func Wrap(err error, code string, meta M, causes ...error) *EventError {
	all := make([]error, 0, 1+len(causes))
	all = append(all, err)
	all = append(all, causes...)

	return &EventError{
		Code:   code,
		Meta:   meta,
		causes: all,
	}
}

func (e *EventError) Error() string {
	if e.Event != "" {
		return fmt.Sprintf("%s event=%s keys=%s", e.Code, e.Event, strings.Join(e.Meta.Keys(), ", "))
	}

	return fmt.Sprintf("%s keys=%s", e.Code, strings.Join(e.Meta.Keys(), ", "))
}

// Unwrap returns every cause, so errors.Is and errors.As traverse the whole
// set, not just the first. Note this is the multi-error form: errors.Unwrap,
// which only knows the single-error one, returns nil on an EventError.
func (e *EventError) Unwrap() []error {
	return e.causes
}

// withEvent returns e with Event set to name if it wasn't already set. Used by
// the bus to stamp the event name onto an error on its way out.
func (e *EventError) withEvent(name string) *EventError {
	if e == nil || e.Event != "" {
		return e
	}

	e.Event = name

	return e
}

// DefaultCoercer is the minimal Coercer: it returns *EventError as-is (via
// errors.As) and otherwise wraps err as code "unknown". It does not know about
// samber/oops or any other third-party error type.
func DefaultCoercer(err error) *EventError {
	if err == nil {
		return nil
	}

	var eventErr *EventError
	if errors.As(err, &eventErr) {
		return eventErr
	}

	return Wrap(err, "unknown", nil)
}
