package core

import (
	"context"
	"log/slog"
	"time"
)

// MetricsCollector is a placeholder for collecting agent metrics.
type MetricsCollector struct{}

// Context holds the shared state and utilities for agents.
type Context struct {
	SessionID string
	State     map[string]interface{} // mutable, shared across agents
	Logger    *slog.Logger
	Metrics   *MetricsCollector
	Cancel    context.CancelFunc
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

// Step can be a single agent or a parallel group.
type Step struct {
	Name   string
	Agents []Agent
}