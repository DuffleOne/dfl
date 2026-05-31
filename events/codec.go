package events

import "encoding/json"

// Codec encodes an event to a payload and decodes a payload back into a typed
// event. It's the events analog of the http package's RequestParser: the
// generic Decode method lets a single codec serve every event shape in the bus,
// and a custom codec is how you swap the wire format (msgpack, protobuf, an
// envelope with a schema id, etc).
//
// Encode takes the Event interface because the value is already in hand at Emit
// time. Decode is generic because the deliver closure knows the concrete type E
// it's decoding into, exactly as adapt calls parser.Parse[Req] in http.
type Codec interface {
	Encode(e Event) (json.RawMessage, error)
	Decode[E Event](payload json.RawMessage) (E, error)
}

// preparable is the optional hook a Codec can satisfy to validate an event
// shape at registration. On type-asserts and calls it so a codec that needs
// per-type setup (schema registration, tag checks) fails at On rather than on
// the first Emit. Mirrors http's preparable/PrepareFor.
type preparable interface {
	PrepareFor[E Event]() error
}

// DefaultCodec is the codec the bus uses when none is set via WithCodec. It
// encodes and decodes events as JSON, so an event's json-tagged fields are its
// wire form. This matches what RegisterEndpoint's HTTP body binding expects.
var DefaultCodec Codec = jsonCodec{}

type jsonCodec struct{}

func (jsonCodec) Encode(e Event) (json.RawMessage, error) {
	payload, err := json.Marshal(e)
	if err != nil {
		return nil, New("encode_failed", M{"error": err.Error()}, err)
	}

	return payload, nil
}

func (jsonCodec) Decode[E Event](payload json.RawMessage) (E, error) {
	var e E

	if err := json.Unmarshal(payload, &e); err != nil {
		return e, New("decode_failed", M{"error": err.Error()}, err)
	}

	return e, nil
}
