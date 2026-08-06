package events

import "encoding/json"

// Event is the one interface every event type must satisfy: EventName is
// the topic it's published and subscribed under, derived by On, Emit, and
// RegisterEndpoint without being repeated at call sites. Events are value
// types by convention, with EventName (and any Validate or URLSafeName) on
// a value receiver, so the bus can call it on a zero value at registration.
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

// Envelope is the wire form of an event: a name, an encoded payload, and
// a bag of string headers, the events analog of *http.Request. Headers
// carry cross-cutting metadata that travels with the event, notably trace
// context: a publish-side Plugin injects, a deliver-side Plugin extracts,
// and each Sink carries them over its transport (the cloud adapters map
// them to native message attributes).
type Envelope struct {
	Name    string            `json:"name"`
	Payload json.RawMessage   `json:"payload"`
	Headers map[string]string `json:"headers,omitempty"`
}
