package main

import (
	"dfl/lib/keychain/windows"
)

func init() {
	kc := windows.Keychain{}

	makeRoot(kc)
}
