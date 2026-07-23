package events

import "encoding/json"

// Codec encodes an event to a payload and decodes a payload back into a typed
// event. It's the events analog of the http package's RequestParser: a single
// codec serves every event shape in the bus.
//
// Encode takes the Event interface because the value is already in hand at
// Emit time. Decode is generic because the deliver closure knows the concrete
// type E it's decoding into, exactly as adapt calls parser.Parse[Req] in http.
//
// Codec is a concrete type rather than an interface because Decode is generic
// over E. Go 1.27 allows type parameters on methods, but only on concrete
// types: interface methods still cannot have them, and a generic method never
// satisfies an interface method. Swap the wire format (msgpack, protobuf, an
// envelope with a schema id) by setting the hooks below rather than by
// implementing the type.
//
// The zero value encodes and decodes as JSON, so an event's json-tagged fields
// are its wire form. That matches what RegisterEndpoint's HTTP body binding
// expects; a codec that moves off JSON needs the endpoint to agree.
type Codec struct {
	// Marshal encodes an event value to its wire payload. Defaults to
	// json.Marshal.
	Marshal func(v any) ([]byte, error)

	// Unmarshal decodes a wire payload into dst, a pointer to the event being
	// decoded. Defaults to json.Unmarshal.
	Unmarshal func(payload []byte, dst any) error

	// Prepare, when non-nil, runs once per event type at On with a pointer to
	// the zero value of that type. A codec needing per-type setup (schema
	// registration, tag checks) can fail there rather than on the first Emit.
	Prepare func(dst any) error
}

// DefaultCodec is the codec the bus uses when none is set via WithCodec.
var DefaultCodec = &Codec{}

// Encode encodes e to its wire payload.
func (c *Codec) Encode(e Event) (json.RawMessage, error) {
	marshal := c.Marshal
	if marshal == nil {
		marshal = json.Marshal
	}

	payload, err := marshal(e)
	if err != nil {
		return nil, New("encode_failed", M{"error": err.Error()}, err)
	}

	return payload, nil
}

// Decode decodes payload into an E.
func (c *Codec) Decode[E Event](payload json.RawMessage) (E, error) {
	var e E

	unmarshal := c.Unmarshal
	if unmarshal == nil {
		unmarshal = json.Unmarshal
	}

	if err := unmarshal(payload, &e); err != nil {
		return e, New("decode_failed", M{"error": err.Error()}, err)
	}

	return e, nil
}

// PrepareFor runs the Prepare hook for E, if one is set. On calls this at
// registration so per-type setup fails there rather than on the first Emit.
func (c *Codec) PrepareFor[E Event]() error {
	if c.Prepare == nil {
		return nil
	}

	var e E

	return c.Prepare(&e)
}
