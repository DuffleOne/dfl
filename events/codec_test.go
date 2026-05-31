package events_test

import (
	"errors"
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
