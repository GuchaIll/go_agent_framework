package queue

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestChannelQueueRoundTrip(t *testing.T) {
	q := NewChannelQueue(10)
	ctx := context.Background()

	job := &Job{
		ID:        "test-1",
		GraphName: "chess_coach",
		SessionID: "sess-1",
		Input:     map[string]interface{}{"fen": "startpos"},
	}

	if err := q.Enqueue(ctx, job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	got, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if got.ID != job.ID {
		t.Fatalf("expected job ID %s, got %s", job.ID, got.ID)
	}
	if got.Status != JobPending {
		t.Fatalf("expected status PENDING, got %s", got.Status)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}

func TestChannelQueueAutoMetadata(t *testing.T) {
	q := NewChannelQueue(10)
	ctx := context.Background()

	job := &Job{GraphName: "doc_qa"}
	if err := q.Enqueue(ctx, job); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	got, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if got.ID == "" {
		t.Fatal("expected auto-generated ID, got empty string")
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
	if got.Status != JobPending {
		t.Fatalf("expected pending status, got %s", got.Status)
	}
}

func TestChannelQueueFIFOOrder(t *testing.T) {
	q := NewChannelQueue(2)
	ctx := context.Background()

	first := &Job{ID: "first"}
	second := &Job{ID: "second"}
	if err := q.Enqueue(ctx, first); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	if err := q.Enqueue(ctx, second); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}

	gotFirst, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue first: %v", err)
	}
	gotSecond, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue second: %v", err)
	}

	if gotFirst.ID != "first" || gotSecond.ID != "second" {
		t.Fatalf("expected FIFO order, got %s then %s", gotFirst.ID, gotSecond.ID)
	}
}

func TestChannelQueueEnqueueContextCancellationOnFullBuffer(t *testing.T) {
	q := NewChannelQueue(1)
	ctx := context.Background()

	if err := q.Enqueue(ctx, &Job{ID: "full"}); err != nil {
		t.Fatalf("enqueue initial: %v", err)
	}

	cancelCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := q.Enqueue(cancelCtx, &Job{ID: "blocked"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestChannelQueueDequeueContextCancellationOnEmptyQueue(t *testing.T) {
	q := NewChannelQueue(10)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := q.Dequeue(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestChannelQueueAckIsNoOp(t *testing.T) {
	q := NewChannelQueue(1)

	if err := q.Ack(context.Background(), &Job{ID: "job-1"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
