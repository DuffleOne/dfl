// Module github.com/duffleone/dfl/http/cher holds the coercer for
// wearemojo/mojo-public-go's cher errors. It is a separate module so the core
// github.com/duffleone/dfl stays free of mojo-public-go's dependency graph
// (mongo, stripe, grpc, the GCP SDKs); depend on this only in a service that
// already speaks cher.
module github.com/duffleone/dfl/http/cher

go 1.27rc2

// The core lives in the same repo and isn't published with these features yet.
replace github.com/duffleone/dfl => ../..

require (
	github.com/duffleone/dfl v0.0.0-00010101000000-000000000000
	github.com/wearemojo/mojo-public-go v0.0.0-20260803142910-68e0dc1cb199
)

require github.com/pkg/errors v0.9.1 // indirect
