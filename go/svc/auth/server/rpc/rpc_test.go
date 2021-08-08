package rpc

import (
	"testing"

	"dfl/svc/auth"
)

func TestInterface(t *testing.T) {
	var svc auth.Service = &RPC{}

	if svc == nil {
		t.Error("not possible")
	}
}
