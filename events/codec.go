package events

import "encoding/json"

// Codec encodes an event to its payload and decodes it back, one codec
// for every event shape, as RequestParser is for http. It's a concrete
// type because Decode is generic over E and Go 1.27's generic methods
// live on concrete types only: swap the wire format via the hooks below,
// not by reimplementing the type. The zero value speaks JSON, the same
// wire form RegisterEndpoint's body binding expects.
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
