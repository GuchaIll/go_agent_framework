package core

import (
	"fmt"
	"strings"
	"sync"
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
	for _, step := range g.steps {
		if len(step.Agents) == 1 {
			if err := step.Agents[0].Run(ctx); err != nil {
				return fmt.Errorf("agent %s failed: %w", step.Agents[0].Name(), err)
			}
		} else {
			var wg sync.WaitGroup
			errs := make([]error, len(step.Agents))
			for i, agent := range step.Agents {
				wg.Add(1)
				go func(idx int, a Agent) {
					defer wg.Done()
					errs[idx] = a.Run(ctx)
				}(i, agent)
			}

			wg.Wait()

			for _, err := range errs {
				if err != nil {
					return fmt.Errorf("parallel step %s: %w", step.Name, err)
				}
			}
		}
	}
	return nil
}
