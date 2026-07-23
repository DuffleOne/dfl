// Module github.com/duffleone/dfl/events/aws holds the AWS transport adapters
// for the events bus (SQS, SNS, EventBridge). It is a separate module so the
// core github.com/duffleone/dfl stays free of the AWS SDK; you depend on this
// only in the service that talks to AWS.
module github.com/duffleone/dfl/events/aws

go 1.27rc2

require (
	github.com/aws/aws-sdk-go-v2 v1.32.8
	github.com/aws/aws-sdk-go-v2/config v1.28.6
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.36.3
	github.com/aws/aws-sdk-go-v2/service/sns v1.33.7
	github.com/aws/aws-sdk-go-v2/service/sqs v1.37.2
	github.com/duffleone/dfl v0.0.0-00010101000000-000000000000
)

require (
	github.com/aws/aws-sdk-go-v2/credentials v1.17.47 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.21 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.27 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.24.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.28.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.2 // indirect
	github.com/aws/smithy-go v1.22.1 // indirect
)

// The core lives in the same repo and isn't published with these features yet.
replace github.com/duffleone/dfl => ../..
