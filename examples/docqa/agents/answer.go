package agents

import (
	"context"
	"fmt"
	"go_agent_framework/contrib/llm"
	"go_agent_framework/core"
	"go_agent_framework/observability"
	"strings"
)

// AnswerAgent composes an answer using retrieved passages and the LLM.
type AnswerAgent struct {
	LLM       llm.LLMClient
	ModelRole string
}

func (a *AnswerAgent) Name() string        { return "answer" }
func (a *AnswerAgent) Description() string { return "Generates an answer from retrieved passages via the LLM." }
func (a *AnswerAgent) Capabilities() core.AgentCapabilities {
	return core.AgentCapabilities{Skills: []string{"llm:generate"}, RAG: []string{"retrieved_passages"}, Model: a.ModelRole}
}

func (a *AnswerAgent) Run(ctx *core.Context) error {
	query, _ := ctx.State["query"].(string)

	// Collect retrieved passages.
	var passages []string
	if raw, ok := ctx.State["retrieved_passages"].([]string); ok {
		passages = raw
	}

	observability.PublishThought(ctx.GraphName, a.Name(), ctx.SessionID, fmt.Sprintf("Generating answer from %d passages.", len(passages)))

	prompt := fmt.Sprintf(
		"Answer the user's question using ONLY the context below.\n\n"+
			"Context:\n%s\n\n"+
			"Question: %s\n\nAnswer:",
		strings.Join(passages, "\n---\n"),
		query,
	)

	observability.PublishSkillUse(ctx.GraphName, a.Name(), ctx.SessionID, "llm:generate", "Asking LLM to answer based on retrieved passages.")

	answer, err := a.LLM.Generate(context.Background(), prompt)
	if err != nil {
		return fmt.Errorf("answer: llm error: %w", err)
	}

	ctx.State["answer"] = answer
	observability.PublishThought(ctx.GraphName, a.Name(), ctx.SessionID, fmt.Sprintf("Answer generated (%d chars).", len(answer)))
	ctx.Logger.Info("answer complete", "answer_len", len(answer))
	return nil
}
