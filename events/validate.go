package events

// Validator validates an event. The bus runs it on the outgoing event at
// Emit and the incoming one at deliver and at the HTTP endpoint, so a
// producer can't publish an invalid event nor a consumer act on one. The
// default calls the event's own Validate method when it has one; swap in
// a struct-tag validator, or any other scheme, with WithValidator.
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
