package events_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/duffleone/dfl/events"
)

func TestDefaultCodecRoundTrip(t *testing.T) {
	payload, err := events.DefaultCodec.Encode(evtPing{Seq: 9})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	got, err := events.DefaultCodec.Decode[evtPing](payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.Seq != 9 {
		t.Errorf("seq = %d, want 9", got.Seq)
	}
}

func TestDefaultCodecDecodeError(t *testing.T) {
	_, err := events.DefaultCodec.Decode[evtPing]([]byte(`{bad`))

	var eventErr *events.EventError
	if !errors.As(err, &eventErr) || eventErr.Code != "decode_failed" {
		t.Fatalf("err = %v, want code decode_failed", err)
	}
}

// TestCodecHooksSwapWireFormat round-trips through a codec whose Marshal and
// Unmarshal are not JSON at all, checking the hooks are what the bus uses.
func TestCodecHooksSwapWireFormat(t *testing.T) {
	c := &events.Codec{
		Marshal: func(v any) ([]byte, error) {
			return []byte(strconv.Itoa(v.(evtPing).Seq)), nil
		},
		Unmarshal: func(payload []byte, dst any) error {
			n, err := strconv.Atoi(string(payload))
			if err != nil {
				return err
			}

			dst.(*evtPing).Seq = n

			return nil
		},
	}

	payload, err := c.Encode(evtPing{Seq: 42})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if string(payload) != "42" {
		t.Errorf("payload = %q, want %q", payload, "42")
	}

	got, err := c.Decode[evtPing](payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.Seq != 42 {
		t.Errorf("seq = %d, want 42", got.Seq)
	}
}

// TestCodecPrepareForRunsHook checks PrepareFor hands the hook a pointer to a
// zero E, which is how a codec discovers the shape it's about to be asked for.
func TestCodecPrepareForRunsHook(t *testing.T) {
	var got any

	c := &events.Codec{
		Prepare: func(dst any) error {
			got = dst

			return nil
		},
	}

	if err := c.PrepareFor[evtPing](); err != nil {
		t.Fatalf("PrepareFor: %v", err)
	}

	if _, ok := got.(*evtPing); !ok {
		t.Fatalf("Prepare got %T, want *evtPing", got)
	}
}

// TestCodecPrepareForPropagatesError checks a rejecting hook surfaces its
// error, which is what makes On panic at registration rather than at Emit.
func TestCodecPrepareForPropagatesError(t *testing.T) {
	want := errors.New("unregistered schema")

	c := &events.Codec{Prepare: func(any) error { return want }}

	if err := c.PrepareFor[evtPing](); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

// TestZeroCodecIsJSON checks the zero value still behaves as the JSON codec,
// so a Codec{} with only one hook set doesn't silently lose the other.
func TestZeroCodecIsJSON(t *testing.T) {
	c := &events.Codec{}

	payload, err := c.Encode(evtPing{Seq: 7})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if string(payload) != `{"seq":7}` {
		t.Errorf("payload = %s, want %s", payload, `{"seq":7}`)
	}
}
