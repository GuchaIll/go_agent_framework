package agents

import (
	"fmt"
	"go_agent_framework/core"
	"go_agent_framework/observability"
	"strings"
)

// FeedbackAgent composes the final coaching response from outputs of the
// parallel agents (analysis, strategy, guard).
type FeedbackAgent struct{}

func (a *FeedbackAgent) Name() string        { return "feedback" }
func (a *FeedbackAgent) Description() string { return "Composes final coaching response from analysis, strategy, and guard outputs." }
func (a *FeedbackAgent) Capabilities() core.AgentCapabilities {
	return core.AgentCapabilities{Agents: []string{"analysis", "strategy", "guard"}}
}

func (a *FeedbackAgent) Run(ctx *core.Context) error {
	observability.PublishThought(ctx.GraphName, a.Name(), ctx.SessionID, "Aggregating results from analysis, strategy, and guard agents.")
	observability.PublishDelegation(ctx.GraphName, a.Name(), ctx.SessionID, "analysis+strategy+guard", "Composing final response from parallel agent outputs.")

	var sb strings.Builder

	// Engine analysis summary
	if metrics, ok := ctx.State["engine_metrics"].(map[string]interface{}); ok {
		sb.WriteString(fmt.Sprintf("Engine evaluation: %v  |  Best move: %v\n",
			metrics["eval"], metrics["best_move"]))
	}

	// Strategic advice
	if advice, ok := ctx.State["strategy_advice"].(string); ok {
		sb.WriteString(fmt.Sprintf("\nCoaching advice:\n%s\n", advice))
	}

	// Move legality feedback
	if legal, ok := ctx.State["move_legal"].(bool); ok {
		move, _ := ctx.State["move"].(string)
		if legal {
			sb.WriteString(fmt.Sprintf("\nYour move %s is legal.", move))
		} else {
			sb.WriteString(fmt.Sprintf("\nYour move %s is NOT legal in this position.", move))
		}
	}

	ctx.State["feedback"] = sb.String()
	observability.PublishThought(ctx.GraphName, a.Name(), ctx.SessionID, fmt.Sprintf("Final coaching response composed (%d chars).", sb.Len()))
	ctx.Logger.Info("feedback composed", "length", sb.Len())
	return nil
}
