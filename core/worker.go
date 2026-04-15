package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// WorkerQueue is the minimal queue interface consumed by Worker.
// It matches contrib/queue.Queue but is declared here so core has no
// dependency on contrib packages.
type WorkerQueue interface {
	Dequeue(ctx context.Context) (WorkerJob, error)
	Ack(ctx context.Context, job WorkerJob) error
}

// WorkerJob is the minimal job interface consumed by Worker.
type WorkerJob interface {
	GetID() string
	GetGraphName() string
	GetSessionID() string
	GetInput() map[string]interface{}
}

// JobStatusUpdater is called by the worker to update job status in a
// persistent store (e.g. DynamoDB). Optional — if nil, status tracking
// is skipped.
type JobStatusUpdater interface {
	UpdateStatus(ctx context.Context, jobID string, status string) error
	SetResult(ctx context.Context, jobID string, result map[string]interface{}, jobErr string) error
}

// Worker consumes jobs from a queue, executes the appropriate graph,
// and persists results via the state store and optional job store.
type Worker struct {
	Graphs map[string]*Graph // graph name → graph instance
	Store  StateStore
	Queue  WorkerQueue
	Jobs   JobStatusUpdater // optional, may be nil
	Logger *slog.Logger
}

// Run starts the worker loop. It blocks until ctx is cancelled.
// Call this in a goroutine (or as the main loop of a worker binary).
func (w *Worker) Run(ctx context.Context) error {
	if w.Logger == nil {
		w.Logger = slog.Default()
	}

	w.Logger.Info("worker started", "graphs", graphNames(w.Graphs))

	for {
		if err := ctx.Err(); err != nil {
			w.Logger.Info("worker shutting down")
			return nil
		}

		if err := w.processOne(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				w.Logger.Info("worker shutting down")
				return nil
			}
			// ErrNoMessages or transient errors — log and continue.
			w.Logger.Warn("worker loop error", "error", err)
			time.Sleep(time.Second)
		}
	}
}

func (w *Worker) processOne(ctx context.Context) error {
	raw, err := w.Queue.Dequeue(ctx)
	if err != nil {
		return err
	}
	if raw == nil {
		return nil
	}

	jobID := raw.GetID()
	graphName := raw.GetGraphName()
	sessionID := raw.GetSessionID()
	input := raw.GetInput()

	log := w.Logger.With("job_id", jobID, "graph", graphName, "session", sessionID)
	log.Info("job received")

	graph, ok := w.Graphs[graphName]
	if !ok {
		log.Error("unknown graph name")
		w.setFailed(ctx, jobID, nil, fmt.Sprintf("unknown graph: %s", graphName))
		_ = w.Queue.Ack(ctx, raw)
		return nil
	}

	// Mark running.
	if w.Jobs != nil {
		_ = w.Jobs.UpdateStatus(ctx, jobID, "RUNNING")
	}

	// Load session state.
	session, err := w.Store.Get(sessionID)
	if err != nil {
		log.Error("failed to load session", "error", err)
		w.setFailed(ctx, jobID, nil, fmt.Sprintf("load session: %v", err))
		_ = w.Queue.Ack(ctx, raw)
		return nil
	}

	// Merge job input into session state.
	for k, v := range input {
		session.State[k] = v
	}

	agentCtx := &Context{
		SessionID:  sessionID,
		State:      session.State,
		Logger:     log,
		StdContext: ctx,
	}

	// Execute the graph.
	start := time.Now()
	graphErr := graph.Run(agentCtx)
	dur := time.Since(start)

	if graphErr != nil {
		log.Error("graph failed", "duration_ms", dur.Milliseconds(), "error", graphErr)
		w.setFailed(ctx, jobID, agentCtx.State, graphErr.Error())
	} else {
		log.Info("graph completed", "duration_ms", dur.Milliseconds())

		// Persist updated session.
		session.State = agentCtx.State
		if err := w.Store.Put(session); err != nil {
			log.Error("failed to save session", "error", err)
		}

		if w.Jobs != nil {
			_ = w.Jobs.SetResult(ctx, jobID, agentCtx.State, "")
		}
	}

	// Always ack — failed jobs are tracked in the job store.
	// SQS DLQ handles infrastructure-level failures (worker crash before ack).
	if err := w.Queue.Ack(ctx, raw); err != nil {
		log.Error("failed to ack job", "error", err)
	}

	return nil
}

func (w *Worker) setFailed(ctx context.Context, jobID string, state map[string]interface{}, errMsg string) {
	if w.Jobs != nil {
		_ = w.Jobs.SetResult(ctx, jobID, state, errMsg)
	}
}

func graphNames(m map[string]*Graph) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	return names
}
