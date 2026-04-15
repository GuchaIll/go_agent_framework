package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"go_agent_framework/core"
	"go_agent_framework/observability"
)

// ToolExecutorAgent iterates over the plan and executes tool calls.
// It reads "plan" from state and writes results to "tool_results".
type ToolExecutorAgent struct {
	Registry *core.ToolRegistry
}

func (a *ToolExecutorAgent) Name() string        { return "tool_executor" }
func (a *ToolExecutorAgent) Description() string { return "Executes tool calls from the planner's output." }
func (a *ToolExecutorAgent) Capabilities() core.AgentCapabilities {
	return core.AgentCapabilities{Tools: []string{"calculate", "get_weather", "search_web", "query_db"}}
}

func (a *ToolExecutorAgent) Run(ctx *core.Context) error {
	planRaw, ok := ctx.State["plan"]
	if !ok {
		return nil // nothing to do
	}

	// Decode plan (may come from JSON round-trip as []interface{}).
	var plan []PlanAction
	switch v := planRaw.(type) {
	case []PlanAction:
		plan = v
	default:
		b, _ := json.Marshal(v)
		if err := json.Unmarshal(b, &plan); err != nil {
			return fmt.Errorf("tool_executor: cannot parse plan: %w", err)
		}
	}

	var results []core.ToolResult
	for _, action := range plan {
		if action.Type != "tool" {
			continue
		}
		observability.PublishToolCall(ctx.GraphName, a.Name(), ctx.SessionID, action.Name, map[string]interface{}{"args": action.Args})
		result := a.Registry.ExecuteTool(context.Background(), core.ToolCall{
			Name: action.Name,
			Args: action.Args,
		})
		if result.Error != "" {
			observability.PublishToolResult(ctx.GraphName, a.Name(), ctx.SessionID, action.Name, "", result.Error)
		} else {
			observability.PublishToolResult(ctx.GraphName, a.Name(), ctx.SessionID, action.Name, result.Output, "")
		}
		results = append(results, result)
		ctx.Logger.Info("tool executed", "tool", action.Name, "has_error", result.Error != "")
	}

	ctx.State["tool_results"] = results
	return nil
}
