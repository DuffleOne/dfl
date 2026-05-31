package events

import (
	"errors"
	"fmt"
	"strings"
)

// EventError is the canonical events error type, the bus analog of the http
// package's ReqError. Code, Event, and Meta are serialised on the wire; Reasons
// is internal-only, traversed by errors.Is and errors.As.
//
// Unlike ReqError there's no StatusCode: a bus has no HTTP status to carry. The
// Event field names the event the error relates to and is stamped by the bus
// when it has the name. RegisterEndpoint maps Code to an HTTP status when it
// projects an EventError back onto a ReqError.
type EventError struct {
	Code    string  `json:"code"`
	Event   string  `json:"event,omitempty"`
	Meta    M       `json:"meta,omitempty"`
	Reasons []error `json:"-"`
}

var _ error = (*EventError)(nil)

// New builds an EventError. reasons (if any) are recorded for errors.Is and
// errors.As traversal.
func New(code string, meta M, reasons ...error) *EventError {
	return &EventError{
		Code:    code,
		Meta:    meta,
		Reasons: reasons,
	}
}

// Wrap builds an EventError that wraps err as its primary cause. Additional
// reasons are recorded after.
func Wrap(err error, code string, meta M, reasons ...error) *EventError {
	all := make([]error, 0, 1+len(reasons))
	all = append(all, err)
	all = append(all, reasons...)

	return &EventError{
		Code:    code,
		Meta:    meta,
		Reasons: all,
	}
}

func (e *EventError) Error() string {
	if e.Event != "" {
		return fmt.Sprintf("%s event=%s keys=%s", e.Code, e.Event, strings.Join(e.Meta.Keys(), ", "))
	}

	return fmt.Sprintf("%s keys=%s", e.Code, strings.Join(e.Meta.Keys(), ", "))
}

// Unwrap returns the primary cause for single-step traversal via errors.Unwrap.
func (e *EventError) Unwrap() error {
	if len(e.Reasons) == 0 {
		return nil
	}

	return e.Reasons[0]
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
