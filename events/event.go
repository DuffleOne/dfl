package events

import "encoding/json"

// Event is the one interface every event type must satisfy. EventName is the
// topic the event is published and subscribed under; it lives with the type so
// On, Emit, and RegisterEndpoint all derive it without the name being repeated
// at the call site.
//
// Events are value types by convention (the opposite of the http package's
// pointer-Req rule). Handlers return only an error, so there's no (nil, err)
// ergonomic to win from pointers, and a value type means EventName is callable
// on a zero value, which the bus relies on to derive the name at registration.
// Put EventName (and any Validate/URLSafeName) on a value receiver.
type Event interface {
	EventName() string
}

// Validatable is the optional self-validation method an event can implement.
// The default Validator calls it; a custom Validator set with WithValidator may
// ignore it. Return a *EventError (via New) to carry a code and field details.
type Validatable interface {
	Validate() error
}

// URLSafeNamer is the optional interface an event can implement to set its HTTP
// endpoint path segment explicitly. When present, RegisterEndpoint uses the
// returned value verbatim as the segment after /events/; otherwise it sanitises
// EventName. It does not affect the bus name used by On and Emit.
type URLSafeNamer interface {
	URLSafeName() string
}

// Envelope is the wire form of an event: a name and an encoded payload. It's
// the events analog of *http.Request, the thing a Sink moves around. The bus
// produces it in Emit (via the Codec) and consumes it in the deliver closure.
type Envelope struct {
	Name    string          `json:"name"`
	Payload json.RawMessage `json:"payload"`
}
