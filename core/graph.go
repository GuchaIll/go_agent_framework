package core

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go_agent_framework/observability"
)

// Graph defines the execution flow as a sequence of steps.
type Graph struct {
	name  string
	steps []Step
}

// NewGraph creates a new named graph.
func NewGraph(name string) *Graph {
	return &Graph{
		name:  name,
		steps: []Step{},
	}
}

// Name returns the graph name.
func (g *Graph) Name() string { return g.name }

// Steps returns a copy of the graph steps.
func (g *Graph) Steps() []Step {
	out := make([]Step, len(g.steps))
	copy(out, g.steps)
	return out
}

// AddSerial adds one agent as a sequential step.
func (g *Graph) AddSerial(agent Agent) *Graph {
	g.steps = append(g.steps, Step{
		Name:   agent.Name(),
		Agents: []Agent{agent},
	})
	return g
}

// AddParallel adds multiple agents that run concurrently.
func (g *Graph) AddParallel(agents ...Agent) *Graph {
	names := make([]string, len(agents))
	for i, agent := range agents {
		names[i] = agent.Name()
	}

	g.steps = append(g.steps, Step{
		Name:   strings.Join(names, "+"),
		Agents: agents,
	})
	return g
}

// Run executes the graph step-by-step, parallelizing where specified.
func (g *Graph) Run(ctx *Context) error {
	if ctx == nil {
		return fmt.Errorf("graph %s failed: nil context", g.name)
	}

	metrics := observability.Default()
	graphCtx := ctx.WithGraph(g.name)
	metrics.IncGraphInFlight(g.name)
	start := time.Now()
	defer func() {
		metrics.DecGraphInFlight(g.name)
	}()

	hub := observability.DefaultHub()
	hub.Publish(observability.DashboardEvent{
		Type:    observability.EventGraphStart,
		Graph:   g.name,
		Session: ctx.SessionID,
	})

	var graphErr error
	defer func() {
		metrics.ObserveGraphRun(g.name, time.Since(start), graphErr)
		status := "success"
		if graphErr != nil {
			status = "error"
		}
		hub.Publish(observability.DashboardEvent{
			Type:       observability.EventGraphEnd,
			Graph:      g.name,
			Session:    ctx.SessionID,
			Status:     status,
			DurationMs: float64(time.Since(start).Milliseconds()),
		})
	}()

	for _, step := range g.steps {
		if len(step.Agents) == 1 {
			if err := runAgent(graphCtx, g.name, step.Agents[0], metrics); err != nil {
				graphErr = fmt.Errorf("agent %s failed: %w", step.Agents[0].Name(), err)
				return graphErr
			}
		} else {
			var wg sync.WaitGroup
			errs := make([]error, len(step.Agents))
			for i, agent := range step.Agents {
				wg.Add(1)
				go func(idx int, a Agent) {
					defer wg.Done()
					errs[idx] = runAgent(graphCtx, g.name, a, metrics)
				}(i, agent)
			}

			wg.Wait()

			for _, err := range errs {
				if err != nil {
					graphErr = fmt.Errorf("parallel step %s: %w", step.Name, err)
					return graphErr
				}
			}
		}
	}
	return nil
}

func runAgent(ctx *Context, graphName string, agent Agent, metrics *observability.Metrics) error {
	hub := observability.DefaultHub()
	agentCtx := ctx.WithAgent(agent.Name())

	hub.Publish(observability.DashboardEvent{
		Type:    observability.EventAgentStart,
		Graph:   graphName,
		Agent:   agent.Name(),
		Session: ctx.SessionID,
	})

	start := time.Now()
	err := agent.Run(agentCtx)
	duration := time.Since(start)
	metrics.ObserveAgentRun(graphName, agent.Name(), duration, err)

	status := "success"
	if err != nil {
		status = "error"
	}
	hub.Publish(observability.DashboardEvent{
		Type:       observability.EventAgentEnd,
		Graph:      graphName,
		Agent:      agent.Name(),
		Session:    ctx.SessionID,
		Status:     status,
		DurationMs: float64(duration.Milliseconds()),
	})

	return err
}
