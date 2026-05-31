package events_test

import (
	"testing"

	"github.com/duffleone/dfl/events"
)

func TestDefaultValidatorCallsValidate(t *testing.T) {
	if err := events.DefaultValidator.Validate(evtUser{Email: ""}); err == nil {
		t.Error("want error for empty email")
	}

	if err := events.DefaultValidator.Validate(evtUser{Email: "x"}); err != nil {
		t.Errorf("want nil for valid event, got %v", err)
	}
}

func TestDefaultValidatorNoopWhenNotValidatable(t *testing.T) {
	if err := events.DefaultValidator.Validate(evtPing{Seq: 1}); err != nil {
		t.Errorf("want nil for non-Validatable event, got %v", err)
	}
}
