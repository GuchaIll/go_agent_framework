package core

import (
	"fmt"
	"sync"
)

// DAGStep is a node in a directed acyclic graph of agents.
type DAGStep struct {
	Name      string
	Agent     Agent
	DependsOn []string // names of steps that must complete first
}

// DAG represents a directed acyclic graph of steps with dependency edges.
type DAG struct {
	steps map[string]*DAGStep
	order []string // insertion order (for determinism)
}

// NewDAG creates an empty DAG.
func NewDAG() *DAG {
	return &DAG{steps: make(map[string]*DAGStep)}
}

// Add registers a step. Returns an error if the name is already taken.
func (d *DAG) Add(step DAGStep) error {
	if _, exists := d.steps[step.Name]; exists {
		return fmt.Errorf("dag: duplicate step %q", step.Name)
	}
	d.steps[step.Name] = &step
	d.order = append(d.order, step.Name)
	return nil
}

// TopologicalSort returns steps grouped into layers. Steps in the same
// layer have all their dependencies satisfied by earlier layers and can
// run in parallel.
func (d *DAG) TopologicalSort() ([][]string, error) {
	// Build in-degree map.
	inDeg := make(map[string]int, len(d.steps))
	dependents := make(map[string][]string, len(d.steps))

	for _, name := range d.order {
		step := d.steps[name]
		inDeg[name] = len(step.DependsOn)
		for _, dep := range step.DependsOn {
			if _, ok := d.steps[dep]; !ok {
				return nil, fmt.Errorf("dag: step %q depends on unknown step %q", name, dep)
			}
			dependents[dep] = append(dependents[dep], name)
		}
	}

	// Kahn's algorithm layer by layer.
	var layers [][]string
	ready := make([]string, 0)
	for _, name := range d.order {
		if inDeg[name] == 0 {
			ready = append(ready, name)
		}
	}

	visited := 0
	for len(ready) > 0 {
		layers = append(layers, ready)
		visited += len(ready)
		var next []string
		for _, name := range ready {
			for _, dep := range dependents[name] {
				inDeg[dep]--
				if inDeg[dep] == 0 {
					next = append(next, dep)
				}
			}
		}
		ready = next
	}

	if visited != len(d.steps) {
		return nil, fmt.Errorf("dag: cycle detected (visited %d of %d steps)", visited, len(d.steps))
	}
	return layers, nil
}

// Run executes the DAG: steps within each layer run in parallel, layers
// execute sequentially.
func (d *DAG) Run(ctx *Context) error {
	layers, err := d.TopologicalSort()
	if err != nil {
		return err
	}

	for _, layer := range layers {
		if len(layer) == 1 {
			step := d.steps[layer[0]]
			if err := step.Agent.Run(ctx); err != nil {
				return fmt.Errorf("dag step %q: %w", step.Name, err)
			}
			continue
		}

		var wg sync.WaitGroup
		errs := make([]error, len(layer))
		for i, name := range layer {
			step := d.steps[name]
			wg.Add(1)
			go func(idx int, s *DAGStep) {
				defer wg.Done()
				errs[idx] = s.Agent.Run(ctx)
			}(i, step)
		}
		wg.Wait()

		for i, err := range errs {
			if err != nil {
				return fmt.Errorf("dag step %q: %w", layer[i], err)
			}
		}
	}
	return nil
}
