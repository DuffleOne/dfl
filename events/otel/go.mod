// Module github.com/duffleone/dfl/events/otel is the OpenTelemetry plugin for
// the events bus. It is a separate module so the core github.com/duffleone/dfl
// stays free of the OTel dependency; depend on this only where you wire tracing.
//
// The sdk and stdouttrace requires are only used by the example.
module github.com/duffleone/dfl/events/otel

go 1.27

require (
	github.com/duffleone/dfl v0.0.0-20260821111237-c605aa1ea07c
	go.opentelemetry.io/otel v1.45.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.45.0
	go.opentelemetry.io/otel/sdk v1.45.0
	go.opentelemetry.io/otel/trace v1.45.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.45.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

// The core lives in the same repo and isn't published with these features yet.
replace github.com/duffleone/dfl => ../..
