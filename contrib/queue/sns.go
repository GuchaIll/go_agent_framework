package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
)

// SNSPublisher publishes jobs to an SNS topic for fan-out to per-graph
// SQS queues via subscription filter policies.
type SNSPublisher struct {
	client   *sns.Client
	topicARN string
}

// NewSNSPublisher creates a publisher bound to a specific SNS topic.
func NewSNSPublisher(client *sns.Client, topicARN string) *SNSPublisher {
	return &SNSPublisher{client: client, topicARN: topicARN}
}

// Publish sends a job to the SNS topic with graph_name as a message
// attribute for subscription filter routing.
func (p *SNSPublisher) Publish(ctx context.Context, job *Job) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	input := &sns.PublishInput{
		TopicArn: &p.topicARN,
		Message:  aws.String(string(body)),
		MessageAttributes: map[string]snstypes.MessageAttributeValue{
			"graph_name": {
				DataType:    aws.String("String"),
				StringValue: aws.String(job.GraphName),
			},
		},
	}

	_, err = p.client.Publish(ctx, input)
	if err != nil {
		return fmt.Errorf("sns publish: %w", err)
	}
	return nil
}
