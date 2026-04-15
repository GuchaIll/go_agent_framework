package core

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ToolParameter describes one parameter a tool accepts.
type ToolParameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string", "number", "boolean", "object"
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// Tool is the interface every callable tool must implement.
type Tool interface {
	Name() string
	Description() string
	Parameters() []ToolParameter
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// ToolCall represents an LLM's request to invoke a tool.
type ToolCall struct {
	ID   string          `json:"id"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"arguments"`
}

// ToolResult holds the output of a tool execution.
type ToolResult struct {
	CallID string `json:"call_id"`
	Name   string `json:"name"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// ToolRegistry is a thread-safe registry of available tools.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewToolRegistry creates an empty registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

// Register adds a tool. Returns an error if a tool with the same name
// is already registered.
func (r *ToolRegistry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name()]; exists {
		return fmt.Errorf("tool %q already registered", t.Name())
	}
	r.tools[t.Name()] = t
	return nil
}

// Get returns a tool by name.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List returns a snapshot of all registered tool names.
func (r *ToolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	return names
}

// Schemas returns a JSON-friendly description of every tool. This is
// typically sent to the LLM so it knows what tools are available.
func (r *ToolRegistry) Schemas() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]map[string]interface{}, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, map[string]interface{}{
			"name":        t.Name(),
			"description": t.Description(),
			"parameters":  t.Parameters(),
		})
	}
	return out
}

// ExecuteTool looks up and executes a tool by name. It is safe for
// concurrent use.
func (r *ToolRegistry) ExecuteTool(ctx context.Context, call ToolCall) ToolResult {
	t, ok := r.Get(call.Name)
	if !ok {
		return ToolResult{CallID: call.ID, Name: call.Name, Error: fmt.Sprintf("unknown tool %q", call.Name)}
	}
	output, err := t.Execute(ctx, call.Args)
	if err != nil {
		return ToolResult{CallID: call.ID, Name: call.Name, Error: err.Error()}
	}
	return ToolResult{CallID: call.ID, Name: call.Name, Output: output}
}

// ToolAgent is a graph-compatible Agent that executes a single tool call
// stored in ctx.State["tool_call"] (a ToolCall) and writes the result to
// ctx.State["tool_result"].
type ToolAgent struct {
	Registry *ToolRegistry
}

func (a *ToolAgent) Name() string { return "tool_executor" }

func (a *ToolAgent) Run(ctx *Context) error {
	raw, ok := ctx.State["tool_call"]
	if !ok {
		return fmt.Errorf("tool_executor: no tool_call in state")
	}

	var call ToolCall
	switch v := raw.(type) {
	case ToolCall:
		call = v
	case map[string]interface{}:
		b, _ := json.Marshal(v)
		if err := json.Unmarshal(b, &call); err != nil {
			return fmt.Errorf("tool_executor: invalid tool_call: %w", err)
		}
	default:
		return fmt.Errorf("tool_executor: unexpected tool_call type %T", raw)
	}

	result := a.Registry.ExecuteTool(context.Background(), call)
	ctx.State["tool_result"] = result
	ctx.Logger.Info("tool executed", "tool", call.Name, "has_error", result.Error != "")
	return nil
}
