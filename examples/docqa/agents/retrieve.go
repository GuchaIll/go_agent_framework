package agents

import (
	"context"
	"fmt"
	"go_agent_framework/contrib/vector"
	"go_agent_framework/core"
	"go_agent_framework/observability"
)

// RetrieveAgent searches the VectorDB for chunks relevant to the user's query.
type RetrieveAgent struct {
	Embedder EmbeddingService
	VectorDB vector.VectorDB
	TopK     int
}

func (a *RetrieveAgent) Name() string        { return "retrieve" }
func (a *RetrieveAgent) Description() string { return "Searches VectorDB for chunks relevant to the query." }
func (a *RetrieveAgent) Capabilities() core.AgentCapabilities {
	return core.AgentCapabilities{RAG: []string{"embedder", "vector_db"}}
}

func (a *RetrieveAgent) Run(ctx *core.Context) error {
	query, _ := ctx.State["query"].(string)
	if query == "" {
		return fmt.Errorf("retrieve: query is required")
	}

	topK := a.TopK
	if topK == 0 {
		topK = 3
	}

	observability.PublishToolCall(ctx.GraphName, a.Name(), ctx.SessionID, "vector_db:search", map[string]interface{}{"query": query, "topK": topK})

	qvec, err := a.Embedder.Embed(context.Background(), query)
	if err != nil {
		return fmt.Errorf("retrieve: embed query: %w", err)
	}

	results, err := a.VectorDB.Search(context.Background(), qvec, topK)
	if err != nil {
		return fmt.Errorf("retrieve: search: %w", err)
	}

	passages := make([]string, len(results))
	for i, r := range results {
		passages[i] = r.Document.Content
	}

	ctx.State["retrieved_passages"] = passages
	observability.PublishToolResult(ctx.GraphName, a.Name(), ctx.SessionID, "vector_db:search", fmt.Sprintf("retrieved %d passages", len(passages)), "")
	ctx.Logger.Info("retrieve complete", "passages", len(passages))
	return nil
}
