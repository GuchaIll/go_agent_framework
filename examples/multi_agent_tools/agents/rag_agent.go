package agents

import (
	"fmt"

	"go_agent_framework/contrib/llm"
	"go_agent_framework/core"
	"go_agent_framework/observability"
)

// RAGAnswerAgent takes the retrieved context and tool results,
// then asks the LLM to produce a final answer.
type RAGAnswerAgent struct {
	LLM       llm.LLMClient
	ModelRole string
}

func (a *RAGAnswerAgent) Name() string        { return "rag_answer" }
func (a *RAGAnswerAgent) Description() string { return "Produces a final answer using retrieved context and the LLM." }
func (a *RAGAnswerAgent) Capabilities() core.AgentCapabilities {
	return core.AgentCapabilities{RAG: []string{"vector_db"}, Skills: []string{"llm:generate"}, Model: a.ModelRole}
}

func (a *RAGAnswerAgent) Run(ctx *core.Context) error {
	query, _ := ctx.State["user_query"].(string)
	ragCtx, _ := ctx.State["rag_context"].(string)

	observability.PublishThought(ctx.GraphName, a.Name(), ctx.SessionID, "Building prompt from RAG context and user query.")

	prompt := fmt.Sprintf(
		"Answer the user's question using the context below.\n\n"+
			"Context:\n%s\n\n"+
			"Question: %s",
		ragCtx, query,
	)

	observability.PublishSkillUse(ctx.GraphName, a.Name(), ctx.SessionID, "llm:generate", "Generating answer from retrieved context.")

	answer, err := a.LLM.Generate(ctx.ToContext(), prompt)
	if err != nil {
		return fmt.Errorf("rag_answer: llm error: %w", err)
	}

	ctx.State["answer"] = answer
	observability.PublishThought(ctx.GraphName, a.Name(), ctx.SessionID, fmt.Sprintf("Answer generated (%d chars).", len(answer)))
	ctx.Logger.Info("rag_answer complete", "answer_len", len(answer))
	return nil
}
