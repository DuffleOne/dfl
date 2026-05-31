package events

// Validator validates an event. The bus runs it on the outgoing event at Emit
// and on the incoming event at deliver and at the HTTP endpoint, so neither a
// producer can publish an invalid event nor a consumer act on one. It's
// pluggable via WithValidator.
//
// The default, DefaultValidator, calls the event's Validate method if it has
// one (the same hand-written convention the http package uses) and is otherwise
// a no-op. Swap in a struct-tag validator, or any other scheme, by implementing
// this interface.
type Validator interface {
	Validate(e Event) error
}

// DefaultValidator calls e.Validate() when e implements Validatable, and
// returns nil otherwise.
var DefaultValidator Validator = validatableValidator{}

type validatableValidator struct{}

func (validatableValidator) Validate(e Event) error {
	if v, ok := e.(Validatable); ok {
		return v.Validate()
	}

	return nil
}

// WithValidator sets the Validator the bus runs on outgoing and incoming
// events. Defaults to DefaultValidator.
func WithValidator(v Validator) Option {
	return func(b *Bus) {
		b.validator = v
	}
}
