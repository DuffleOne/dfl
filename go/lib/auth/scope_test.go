package auth

import (
	"testing"
)

var suite = []struct {
	Name   string
	Action string
	Scopes string
	Result bool
}{
	{
		Name:   "match up",
		Action: "auth:login",
		Scopes: "auth:*",
		Result: true,
	},
	{
		Name:   "match exact",
		Action: "auth:*",
		Scopes: "auth:*",
		Result: true,
	},
	{
		Name:   "match within category",
		Action: "short:upload short:moderate",
		Scopes: "short:*",
		Result: true,
	},
	{
		Name:   "match within",
		Action: "short:upload short:meta",
		Scopes: "short:upload short:meta short:moderate",
		Result: true,
	},
	{
		Name:   "do no match down",
		Action: "auth:*",
		Scopes: "auth:login",
		Result: false,
	},
	{
		Name:   "do not match unrelated",
		Action: "auth:list",
		Scopes: "auth:login",
		Result: false,
	},
	{
		Name:   "match in set",
		Action: "auth:login",
		Scopes: "auth:login auth:list dflimg:upload",
		Result: true,
	},
	{
		Name:   "super root works",
		Action: "auth:login",
		Scopes: "*:*",
		Result: true,
	},
}

func TestCan(t *testing.T) {
	for _, test := range suite {
		res := Can(test.Action, test.Scopes)
		if res != test.Result {
			t.Errorf("unexpected \"Can\" result in test \"%s\", expected %t, got %t", test.Name, test.Result, res)
		}
	}
}
