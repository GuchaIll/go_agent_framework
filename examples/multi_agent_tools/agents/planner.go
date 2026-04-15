package agents

import (
	"encoding/json"
	"fmt"

	"go_agent_framework/contrib/llm"
	"go_agent_framework/core"
	"go_agent_framework/observability"
)

// PlannerAgent asks the LLM to decide which tools/skills to call.
// It reads "user_query" from state and writes a plan to "plan".
type PlannerAgent struct {
	LLM       llm.LLMClient
	Tools     *core.ToolRegistry
	ModelRole string
}

// PlanAction is one step in the planner's output.
type PlanAction struct {
	Type string          `json:"type"` // "tool" or "skill"
	Name string          `json:"name"` // tool or skill name
	Args json.RawMessage `json:"args,omitempty"`
}

func (a *PlannerAgent) Name() string        { return "planner" }
func (a *PlannerAgent) Description() string { return "Asks the LLM to decide which tools and skills to call." }
func (a *PlannerAgent) Capabilities() core.AgentCapabilities {
	return core.AgentCapabilities{Skills: []string{"llm:generate"}, Tools: []string{"tool_registry"}, Model: a.ModelRole}
}

func (a *PlannerAgent) Run(ctx *core.Context) error {
	query, _ := ctx.State["user_query"].(string)
	if query == "" {
		return fmt.Errorf("planner: no user_query in state")
	}

	schemas := a.Tools.Schemas()
	schemasJSON, _ := json.Marshal(schemas)

	observability.PublishThought(ctx.GraphName, a.Name(), ctx.SessionID, fmt.Sprintf("Planning actions for query: %s", query))

	prompt := fmt.Sprintf(
		"You are a planning agent. Given the user query and available tools, "+
			"return a JSON array of actions to take.\n\n"+
			"Available tools:\n%s\n\n"+
			"User query: %s\n\n"+
			"Return JSON array of {\"type\":\"tool\",\"name\":\"...\",\"args\":{...}}",
		schemasJSON, query,
	)

	observability.PublishSkillUse(ctx.GraphName, a.Name(), ctx.SessionID, "llm:generate", "Asking LLM to produce a tool-call plan.")

	resp, err := a.LLM.Generate(ctx.ToContext(), prompt)
	if err != nil {
		return fmt.Errorf("planner: llm error: %w", err)
	}

	var plan []PlanAction
	if err := json.Unmarshal([]byte(resp), &plan); err != nil {
		// If the LLM returns non-JSON, store raw.
		ctx.State["plan_raw"] = resp
		ctx.State["plan"] = []PlanAction{}
		observability.PublishThought(ctx.GraphName, a.Name(), ctx.SessionID, "LLM returned non-JSON plan; storing raw output.")
		ctx.Logger.Warn("planner: llm returned non-JSON plan", "raw", resp)
		return nil
	}

	ctx.State["plan"] = plan
	observability.PublishThought(ctx.GraphName, a.Name(), ctx.SessionID, fmt.Sprintf("Plan created with %d action(s).", len(plan)))
	ctx.Logger.Info("planner complete", "actions", len(plan))
	return nil
}
