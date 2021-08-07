package main

import (
	"dfl/lib/keychain/darwin"
)

func init() {
	kc := darwin.Keychain{}

	makeRoot(kc)
}
