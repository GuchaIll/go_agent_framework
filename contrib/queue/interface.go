package queue

import (
	"context"
	"errors"
	"time"
)

// ErrNoMessages is returned by Dequeue when no messages are available.
// Workers should treat this as a normal "try again" condition.
var ErrNoMessages = errors.New("queue: no messages available")

// JobStatus tracks a job through its lifecycle.
type JobStatus string

const (
	JobPending   JobStatus = "PENDING"
	JobRunning   JobStatus = "RUNNING"
	JobCompleted JobStatus = "COMPLETED"
	JobFailed    JobStatus = "FAILED"
)

// Job represents a unit of work to be processed by a worker.
type Job struct {
	ID        string                 `json:"id"`
	GraphName string                 `json:"graph_name"`
	SessionID string                 `json:"session_id"`
	Status    JobStatus              `json:"status"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Result    map[string]interface{} `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`

	// ReceiptHandle is SQS-specific metadata needed to Ack the message.
	// Channel-based queue ignores this.
	ReceiptHandle string `json:"-"`
}

// Queue is the abstraction consumed by both the HTTP handler (producer)
// and the worker loop (consumer).
type Queue interface {
	// Enqueue submits a job for processing.
	Enqueue(ctx context.Context, job *Job) error

	// Dequeue blocks (or long-polls) until a job is available.
	// Returns ErrNoMessages when no work is available (not a failure).
	Dequeue(ctx context.Context) (*Job, error)

	// Ack acknowledges that the job has been processed and removes it
	// from the queue.
	Ack(ctx context.Context, job *Job) error
}
