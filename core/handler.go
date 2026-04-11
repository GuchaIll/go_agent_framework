package core

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// Orchestrator ties a graph to a state store and serves HTTP requests.
type Orchestrator struct {
	Graph   *Graph
	Store   StateStore
	workers chan struct{} // concurrency limiter
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

// HandleRequest is an http.HandlerFunc that decodes JSON input, executes the
// graph, persists state, and returns selected output fields.
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
		SessionID: sessionID,
		State:     session.State,
		Logger:    slog.Default(),
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
