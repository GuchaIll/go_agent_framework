package agents

import (
	"context"
	"fmt"
	"go_agent_framework/contrib/vector"
	"go_agent_framework/core"
	"go_agent_framework/observability"
)

// EmbeddingService converts text into a vector embedding.
type EmbeddingService interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// EmbedAgent embeds each text chunk and inserts documents into a VectorDB.
type EmbedAgent struct {
	Embedder EmbeddingService
	VectorDB vector.VectorDB
}

func (a *EmbedAgent) Name() string        { return "embed" }
func (a *EmbedAgent) Description() string { return "Embeds each chunk and inserts vectors into VectorDB." }
func (a *EmbedAgent) Capabilities() core.AgentCapabilities {
	return core.AgentCapabilities{RAG: []string{"embedder", "vector_db"}}
}

func (a *EmbedAgent) Run(ctx *core.Context) error {
	chunksRaw, _ := ctx.State["chunks"].([]string)
	if len(chunksRaw) == 0 {
		return fmt.Errorf("embed: no chunks found in state")
	}

	observability.PublishToolCall(ctx.GraphName, a.Name(), ctx.SessionID, "embedder:embed", map[string]interface{}{"chunks": len(chunksRaw)})

	docs := make([]vector.Document, 0, len(chunksRaw))
	for i, chunk := range chunksRaw {
		vec, err := a.Embedder.Embed(context.Background(), chunk)
		if err != nil {
			return fmt.Errorf("embed: chunk %d: %w", i, err)
		}
		docs = append(docs, vector.Document{
			ID:      fmt.Sprintf("chunk-%d", i),
			Content: chunk,
			Vector:  vec,
		})
	}

	if err := a.VectorDB.Insert(context.Background(), docs); err != nil {
		return fmt.Errorf("embed: insert: %w", err)
	}

	ctx.State["embedded_count"] = len(docs)
	observability.PublishToolResult(ctx.GraphName, a.Name(), ctx.SessionID, "embedder:embed", fmt.Sprintf("embedded %d documents", len(docs)), "")
	ctx.Logger.Info("embed complete", "docs", len(docs))
	return nil
}

// MockEmbedder is a test double that returns a fixed-dimension zero vector.
type MockEmbedder struct {
	Dim int
}

func (m *MockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	dim := m.Dim
	if dim == 0 {
		dim = 128
	}
	return make([]float32, dim), nil
}
