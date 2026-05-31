// Module github.com/duffleone/dfl/events/otel is the OpenTelemetry plugin for
// the events bus. It is a separate module so the core github.com/duffleone/dfl
// stays free of the OTel dependency; depend on this only where you wire tracing.
//
// go.sum is not checked in: the core targets a Go with generic methods the
// current toolchain doesn't parse, so `go mod tidy` can't run yet. Once the
// core builds, run `go mod tidy` here to populate go.sum. The sdk and
// stdouttrace requires are only used by the example.
module github.com/duffleone/dfl/events/otel

go 1.26.2

require (
	github.com/duffleone/dfl v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/otel v1.43.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.32.0
	go.opentelemetry.io/otel/sdk v1.43.0
	go.opentelemetry.io/otel/trace v1.43.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
)

// The core lives in the same repo and isn't published with these features yet.
replace github.com/duffleone/dfl => ../..
