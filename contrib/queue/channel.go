package queue

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"
)

// ChannelQueue implements Queue using a buffered Go channel.
// Use for local development, testing, and single-process deployments.
type ChannelQueue struct {
	ch chan *Job
}

// NewChannelQueue creates a queue with the given buffer capacity.
func NewChannelQueue(capacity int) *ChannelQueue {
	return &ChannelQueue{ch: make(chan *Job, capacity)}
}

func (q *ChannelQueue) Enqueue(ctx context.Context, job *Job) error {
	if job.ID == "" {
		job.ID = generateID()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}
	job.Status = JobPending

	select {
	case q.ch <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *ChannelQueue) Dequeue(ctx context.Context) (*Job, error) {
	select {
	case job := <-q.ch:
		return job, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Ack is a no-op for channel queues — items are removed on receive.
func (q *ChannelQueue) Ack(_ context.Context, _ *Job) error {
	return nil
}

func generateID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b)
}
