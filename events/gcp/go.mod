// Module github.com/duffleone/dfl/events/gcp holds the Google Cloud transport
// adapter for the events bus (Pub/Sub). It is a separate module so the core
// github.com/duffleone/dfl stays free of the Pub/Sub SDK; depend on this only
// in the service that talks to GCP.
//
// go.sum is not checked in: the core targets a Go with generic methods that the
// current toolchain doesn't parse, so `go mod tidy` can't run yet. Once the
// core builds, run `go mod tidy` here to pin the version and populate go.sum.
module github.com/duffleone/dfl/events/gcp

go 1.26.2

require (
	cloud.google.com/go/pubsub v1.45.3
	github.com/duffleone/dfl v0.0.0-00010101000000-000000000000
)

// The core lives in the same repo and isn't published with these features yet.
replace github.com/duffleone/dfl => ../..
