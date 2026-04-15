package core

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// JobEnqueuer is used by HandleAsyncRequest to submit jobs.
// It matches the Enqueue signature from contrib/queue.Queue.
type JobEnqueuer interface {
	Enqueue(ctx context.Context, job AsyncJob) error
}

// AsyncJob is the minimal job interface for async submission.
type AsyncJob interface {
	SetID(id string)
	SetSessionID(sid string)
}

// JobGetter retrieves a job by ID for status polling.
type JobGetter interface {
	GetJob(ctx context.Context, jobID string) (interface{}, error)
}

// Orchestrator ties a graph to a state store and serves HTTP requests.
type Orchestrator struct {
	Graph    *Graph
	Store    StateStore
	Enqueuer JobEnqueuer // optional, needed for async mode
	JobStore JobGetter   // optional, needed for async polling
	workers  chan struct{}
}

// NewOrchestrator creates an orchestrator with the given concurrency limit.
// maxConcurrency controls how many graph executions can run simultaneously.
func NewOrchestrator(graph *Graph, store StateStore, maxConcurrency int) *Orchestrator {
	return &Orchestrator{
		Graph:   graph,
		Store:   store,
		workers: make(chan struct{}, maxConcurrency),
	}
}

// HandleRequest is the synchronous path: decode JSON, run graph, return result.
func (o *Orchestrator) HandleRequest(w http.ResponseWriter, r *http.Request) {
	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	// Acquire worker slot.
	o.workers <- struct{}{}
	defer func() { <-o.workers }()

	// Load previous state.
	session, err := o.Store.Get(sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load session: %v", err), http.StatusInternalServerError)
		return
	}

	// Merge input into state (input overrides existing keys).
	for k, v := range input {
		session.State[k] = v
	}

	ctx := &Context{
		SessionID:  sessionID,
		State:      session.State,
		Logger:     slog.Default(),
		StdContext: r.Context(),
	}

	// Run graph.
	if err := o.Graph.Run(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save updated state.
	session.State = ctx.State
	if err := o.Store.Put(session); err != nil {
		http.Error(w, fmt.Sprintf("failed to save session: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Session-ID", sessionID)
	json.NewEncoder(w).Encode(ctx.State)
}

// AsyncRequest is the JSON body for HandleAsyncRequest.
type AsyncRequest struct {
	GraphName string                 `json:"graph_name"`
	SessionID string                 `json:"session_id,omitempty"`
	Input     map[string]interface{} `json:"input"`
}

// HandleAsyncRequest enqueues a job and returns 202 Accepted with the job ID.
// The client can poll HandleJobStatus for results.
func (o *Orchestrator) HandleAsyncRequest(w http.ResponseWriter, r *http.Request) {
	if o.Enqueuer == nil || o.JobStore == nil {
		http.Error(w, "async mode not configured", http.StatusServiceUnavailable)
		return
	}

	var req AsyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.GraphName == "" {
		http.Error(w, "graph_name is required", http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		req.SessionID = generateSessionID()
	}

	jobID := generateSessionID()

	// Create a concrete job envelope that satisfies AsyncJob.
	job := &asyncJobEnvelope{
		id:        jobID,
		graphName: req.GraphName,
		sessionID: req.SessionID,
		input:     req.Input,
	}

	if err := o.Enqueuer.Enqueue(r.Context(), job); err != nil {
		http.Error(w, fmt.Sprintf("failed to enqueue: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"job_id":     jobID,
		"session_id": req.SessionID,
		"status":     "PENDING",
	})
}

// HandleJobStatus returns the current status of an async job.
// Expects query parameter: ?job_id=...
func (o *Orchestrator) HandleJobStatus(w http.ResponseWriter, r *http.Request) {
	if o.JobStore == nil {
		http.Error(w, "async mode not configured", http.StatusServiceUnavailable)
		return
	}

	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		http.Error(w, "job_id query parameter is required", http.StatusBadRequest)
		return
	}

	job, err := o.JobStore.GetJob(r.Context(), jobID)
	if err != nil {
		http.Error(w, fmt.Sprintf("job not found: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// PickOutputFields returns only the named keys from a state map.
func PickOutputFields(state map[string]interface{}, keys ...string) map[string]interface{} {
	out := make(map[string]interface{}, len(keys))
	for _, k := range keys {
		if v, ok := state[k]; ok {
			out[k] = v
		}
	}
	return out
}

// generateSessionID returns a UUID v4 string using crypto/rand.
func generateSessionID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// asyncJobEnvelope is a minimal struct used to pass job data to the enqueuer.
type asyncJobEnvelope struct {
	id        string
	graphName string
	sessionID string
	input     map[string]interface{}
}

func (j *asyncJobEnvelope) SetID(id string)         { j.id = id }
func (j *asyncJobEnvelope) SetSessionID(sid string) { j.sessionID = sid }
