package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// SQSQueue implements Queue backed by an AWS SQS queue.
type SQSQueue struct {
	client      *sqs.Client
	queueURL    string
	WaitSeconds int32 // long-poll duration (max 20, default 20)
}

// NewSQSQueue creates a queue bound to a specific SQS queue URL.
func NewSQSQueue(client *sqs.Client, queueURL string) *SQSQueue {
	return &SQSQueue{
		client:      client,
		queueURL:    queueURL,
		WaitSeconds: 20,
	}
}

func (q *SQSQueue) Enqueue(ctx context.Context, job *Job) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}

	input := &sqs.SendMessageInput{
		QueueUrl:    &q.queueURL,
		MessageBody: aws.String(string(body)),
		MessageAttributes: map[string]sqstypes.MessageAttributeValue{
			"graph_name": {
				DataType:    aws.String("String"),
				StringValue: aws.String(job.GraphName),
			},
		},
	}

	_, err = q.client.SendMessage(ctx, input)
	if err != nil {
		return fmt.Errorf("sqs send: %w", err)
	}
	return nil
}

func (q *SQSQueue) Dequeue(ctx context.Context) (*Job, error) {
	output, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:              &q.queueURL,
		MaxNumberOfMessages:   1,
		WaitTimeSeconds:       q.WaitSeconds,
		MessageAttributeNames: []string{"All"},
	})
	if err != nil {
		return nil, fmt.Errorf("sqs receive: %w", err)
	}

	if len(output.Messages) == 0 {
		return nil, ErrNoMessages
	}

	msg := output.Messages[0]
	var job Job
	if err := json.Unmarshal([]byte(*msg.Body), &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}

	job.ReceiptHandle = *msg.ReceiptHandle
	return &job, nil
}

func (q *SQSQueue) Ack(ctx context.Context, job *Job) error {
	_, err := q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &q.queueURL,
		ReceiptHandle: &job.ReceiptHandle,
	})
	if err != nil {
		return fmt.Errorf("sqs delete: %w", err)
	}
	return nil
}
