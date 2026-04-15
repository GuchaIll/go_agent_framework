package core

import (
	"context"
	"log/slog"
	"time"

	"go_agent_framework/observability"
)

// MetricsCollector is a placeholder for collecting agent metrics.
type MetricsCollector struct{}

// Context holds the shared state and utilities for agents.
type Context struct {
	SessionID  string
	GraphName  string
	AgentName  string
	State      map[string]interface{} // mutable, shared across agents
	Logger     *slog.Logger
	Metrics    *MetricsCollector
	StdContext context.Context
	Cancel     context.CancelFunc
}

// Agent defines the interface that all agents must implement.
type Agent interface {
	Name() string
	Run(ctx *Context) error
}

// Session represents a persisted agent session with versioned state.
type Session struct {
	ID        string
	State     map[string]interface{}
	Version   int
	UpdatedAt time.Time
}

// StateStore is the interface for session persistence backends.
type StateStore interface {
	Get(id string) (*Session, error)
	Put(session *Session) error
}

// ToContext returns a standard library context.Context enriched with the
// current session, graph, and agent labels.
func (c *Context) ToContext() context.Context {
	base := c.StdContext
	if base == nil {
		base = context.Background()
	}

	base = observability.WithSession(base, c.SessionID)
	base = observability.WithGraph(base, c.GraphName)
	base = observability.WithAgent(base, c.AgentName)
	return base
}

// WithGraph returns a shallow copy of the context tagged with graph metadata.
func (c *Context) WithGraph(name string) *Context {
	if c == nil {
		return &Context{GraphName: name}
	}
	clone := *c
	clone.GraphName = name
	return &clone
}

// WithAgent returns a shallow copy of the context tagged with agent metadata.
func (c *Context) WithAgent(name string) *Context {
	if c == nil {
		return &Context{AgentName: name}
	}
	clone := *c
	clone.AgentName = name
	if clone.Logger != nil {
		clone.Logger = clone.Logger.With("agent", name)
	}
	return &clone
}

// Step can be a single agent or a parallel group.
type Step struct {
	Name   string
	Agents []Agent
}
