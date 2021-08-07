package rpc

import (
	"testing"

	"dfl/svc/short"
)

func TestInterface(t *testing.T) {
	var svc short.Service = &RPC{}

	if svc == nil {
		t.Error("not possible")
	}
}
