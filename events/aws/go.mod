// Module github.com/duffleone/dfl/events/aws holds the AWS transport adapters
// for the events bus (SQS, SNS, EventBridge). It is a separate module so the
// core github.com/duffleone/dfl stays free of the AWS SDK; you depend on this
// only in the service that talks to AWS.
//
// go.sum is not checked in: the core targets a Go with generic methods that the
// current toolchain doesn't parse, so `go mod tidy` can't run yet. Once the
// core builds, run `go mod tidy` here to pin the SDK versions and populate
// go.sum. The require versions below are a starting point.
module github.com/duffleone/dfl/events/aws

go 1.26.2

require (
	github.com/aws/aws-sdk-go-v2 v1.32.6
	github.com/aws/aws-sdk-go-v2/config v1.28.6
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.36.3
	github.com/aws/aws-sdk-go-v2/service/sns v1.33.7
	github.com/aws/aws-sdk-go-v2/service/sqs v1.37.2
	github.com/duffleone/dfl v0.0.0-00010101000000-000000000000
)

// The core lives in the same repo and isn't published with these features yet.
replace github.com/duffleone/dfl => ../..
