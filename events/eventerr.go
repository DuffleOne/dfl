package events

import (
	"errors"
	"fmt"
	"strings"
)

// EventError is the canonical events error type, the bus analog of
// ReqError: Code, Event, and Meta serialise, causes stay internal for
// errors.Is and errors.As. There's no status, a bus having none to carry;
// RegisterEndpoint decides one from Code when projecting an EventError
// back onto a ReqError. Event names the event concerned, stamped by the
// bus when it has the name.
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

// withEvent returns e with Event set to name if it wasn't already. It
// copies rather than mutates, as ReqError's With* methods do: handlers
// return package-level sentinels (NotFound, say), and stamping a shared
// value would race deliveries and leak one event's name into another's.
func (e *EventError) withEvent(name string) *EventError {
	if e == nil || e.Event != "" {
		return e
	}

	out := *e
	out.Event = name

	return &out
}

// DefaultCoercer is the minimal Coercer: it returns *EventError as-is (via
// errors.As) and otherwise wraps err as code "unknown". It does not know about
// samber/oops or any other third-party error type.
func DefaultCoercer(err error) *EventError {
	if err == nil {
		return nil
	}

	if eventErr, ok := errors.AsType[*EventError](err); ok {
		return eventErr
	}

	return Wrap(err, "unknown", nil)
}
