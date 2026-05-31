// Package sns provides an events publisher and an HTTP push ingress for Amazon
// SNS. Publish sends an event to its topic; the push sink receives SNS HTTP
// notifications and dispatches them to the registered handlers, so an SNS
// subscription can deliver straight into your bus.
//
// SNS does not let you pull. For delivery you either push to an HTTP endpoint
// (this package) or subscribe an SQS queue to the topic and pull that with the
// sqs package. A common shape is: publish with this package, receive with sqs.
package sns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/duffleone/dfl/events"
)

const eventAttr = "event"

// PublishAPI is the slice of *sns.Client the publisher calls.
type PublishAPI interface {
	Publish(ctx context.Context, in *sns.PublishInput, optFns ...func(*sns.Options)) (*sns.PublishOutput, error)
}

// Publisher publishes events to SNS topics, one topic per event name. topics
// maps an event name to its topic ARN.
type Publisher struct {
	client PublishAPI
	topics map[string]string
}

// NewPublisher builds a Publisher. topics maps event name to topic ARN.
func NewPublisher(client PublishAPI, topics map[string]string) *Publisher {
	return &Publisher{client: client, topics: topics}
}

// Publish sends env to the topic mapped from its name. It returns once SNS has
// accepted the message.
func (p *Publisher) Publish(ctx context.Context, env events.Envelope) error {
	arn, ok := p.topics[env.Name]
	if !ok {
		return events.New("no_topic", events.M{"event": env.Name})
	}

	_, err := p.client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(arn),
		Message:  aws.String(string(env.Payload)),
		MessageAttributes: map[string]types.MessageAttributeValue{
			eventAttr: {DataType: aws.String("String"), StringValue: aws.String(env.Name)},
		},
	})
	if err != nil {
		return events.Wrap(err, "publish_failed", events.M{"event": env.Name})
	}

	return nil
}

// PushSink is an events.Sink that publishes to SNS and receives via an SNS HTTP
// push subscription. Register it with the bus (Publish from the Publisher,
// Subscribe from the Dispatcher), and mount it as an http.Handler so SNS
// notifications reach your handlers.
type PushSink struct {
	*events.Dispatcher
	*Publisher

	httpClient  *http.Client
	autoConfirm bool
}

var (
	_ events.Sink  = (*PushSink)(nil)
	_ http.Handler = (*PushSink)(nil)
)

// PushOption configures a PushSink.
type PushOption func(*PushSink)

// WithAutoConfirm controls whether the sink confirms an SNS
// SubscriptionConfirmation by fetching its SubscribeURL. Defaults to true, but
// only URLs on an amazonaws.com host are fetched. Set false to confirm
// subscriptions out of band.
func WithAutoConfirm(on bool) PushOption {
	return func(s *PushSink) { s.autoConfirm = on }
}

// NewPushSink builds a push sink that publishes to the given topics and accepts
// SNS notifications over HTTP.
func NewPushSink(client PublishAPI, topics map[string]string, opts ...PushOption) *PushSink {
	s := &PushSink{
		Dispatcher:  events.NewDispatcher(),
		Publisher:   NewPublisher(client, topics),
		httpClient:  http.DefaultClient,
		autoConfirm: true,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// snsMessage is the JSON SNS POSTs to an HTTP subscription.
type snsMessage struct {
	Type              string `json:"Type"`
	Message           string `json:"Message"`
	SubscribeURL      string `json:"SubscribeURL"`
	MessageAttributes map[string]struct {
		Value string `json:"Value"`
	} `json:"MessageAttributes"`
}

// ServeHTTP handles an SNS HTTP delivery: it confirms a subscription handshake
// and dispatches a notification to the registered handlers. A 2xx tells SNS the
// message is handled; a 5xx makes SNS retry.
//
// Note: this does not verify the SNS message signature. In production you
// should verify it (and restrict the topic) before trusting a message.
func (s *PushSink) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var msg snsMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "invalid sns message", http.StatusBadRequest)

		return
	}

	switch msg.Type {
	case "SubscriptionConfirmation":
		if s.autoConfirm && isAWSHost(msg.SubscribeURL) {
			if _, err := s.httpClient.Get(msg.SubscribeURL); err != nil {
				http.Error(w, "confirm failed", http.StatusBadGateway)

				return
			}
		}

		w.WriteHeader(http.StatusOK)

	case "Notification":
		name := msg.MessageAttributes[eventAttr].Value
		env := events.Envelope{Name: name, Payload: []byte(msg.Message)}

		if err := s.Dispatch(r.Context(), env); err != nil {
			http.Error(w, "handler failed", http.StatusInternalServerError)

			return
		}

		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusOK)
	}
}

// isAWSHost guards the confirmation fetch so only SNS URLs are followed.
func isAWSHost(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}

	return strings.HasSuffix(u.Hostname(), ".amazonaws.com")
}
