// Module github.com/duffleone/dfl/events/aws holds the AWS transport adapters
// for the events bus (SQS, SNS, EventBridge). It is a separate module so the
// core github.com/duffleone/dfl stays free of the AWS SDK; you depend on this
// only in the service that talks to AWS.
module github.com/duffleone/dfl/events/aws

go 1.27

require (
	github.com/aws/aws-sdk-go-v2 v1.47.3
	github.com/aws/aws-sdk-go-v2/config v1.32.39
	github.com/aws/aws-sdk-go-v2/service/eventbridge v1.55.3
	github.com/aws/aws-sdk-go-v2/service/sns v1.42.7
	github.com/aws/aws-sdk-go-v2/service/sqs v1.52.4
	github.com/duffleone/dfl v0.0.0-20260821111237-c605aa1ea07c
)

require (
	github.com/aws/aws-sdk-go-v2/credentials v1.19.38 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.39 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.39 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.5.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.33.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.38.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.45.8 // indirect
	github.com/aws/smithy-go v1.28.5 // indirect
)

// The core lives in the same repo and isn't published with these features yet.
replace github.com/duffleone/dfl => ../..
