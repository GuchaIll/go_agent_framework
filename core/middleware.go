package core

import (
	"context"
	"fmt"
	"time"
)

// --- LoggingAgent: wraps an agent with timing and log output ---

// LoggingAgent logs the start, duration, and outcome of an inner agent.
type LoggingAgent struct {
	Inner Agent
}

func (l *LoggingAgent) Name() string { return l.Inner.Name() }

func (l *LoggingAgent) Run(ctx *Context) error {
	ctx.Logger.Info("agent started", "name", l.Inner.Name())
	start := time.Now()
	err := l.Inner.Run(ctx)
	dur := time.Since(start)
	if err != nil {
		ctx.Logger.Error("agent failed", "name", l.Inner.Name(), "duration_ms", dur.Milliseconds(), "error", err)
	} else {
		ctx.Logger.Info("agent finished", "name", l.Inner.Name(), "duration_ms", dur.Milliseconds())
	}
	return err
}

// --- RetryableAgent: retries an inner agent with linear backoff ---

// RetryableAgent retries the inner agent up to MaxRetry times with linear backoff.
type RetryableAgent struct {
	Inner    Agent
	MaxRetry int
	Backoff  time.Duration
}

func (r *RetryableAgent) Name() string { return r.Inner.Name() }

func (r *RetryableAgent) Run(ctx *Context) error {
	var err error
	for i := 0; i <= r.MaxRetry; i++ {
		if err = r.Inner.Run(ctx); err == nil {
			return nil
		}
		if i < r.MaxRetry {
			time.Sleep(r.Backoff * time.Duration(i+1))
		}
	}
	return fmt.Errorf("agent %s failed after %d retries: %w", r.Inner.Name(), r.MaxRetry, err)
}

// --- ConditionalAgent: branches execution based on a condition ---

// ConditionalAgent evaluates a condition and dispatches to Then or Else.
type ConditionalAgent struct {
	ConditionName string
	Condition     func(ctx *Context) bool
	Then          Agent
	Else          Agent // may be nil
}

func (c *ConditionalAgent) Name() string { return c.ConditionName }

func (c *ConditionalAgent) Run(ctx *Context) error {
	if c.Condition(ctx) {
		return c.Then.Run(ctx)
	}
	if c.Else != nil {
		return c.Else.Run(ctx)
	}
	return nil
}

// --- TimeoutAgent: enforces a deadline on an inner agent ---

// TimeoutAgent wraps an agent with a context timeout.
type TimeoutAgent struct {
	Inner   Agent
	Timeout time.Duration
}

func (t *TimeoutAgent) Name() string { return t.Inner.Name() }

func (t *TimeoutAgent) Run(ctx *Context) error {
	dctx, cancel := context.WithTimeout(context.Background(), t.Timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- t.Inner.Run(ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-dctx.Done():
		return fmt.Errorf("agent %s timed out after %v", t.Inner.Name(), t.Timeout)
	}
}
