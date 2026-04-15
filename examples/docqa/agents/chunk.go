package agents

import (
	"fmt"
	"go_agent_framework/core"
	"go_agent_framework/observability"
	"strings"
)

// ChunkAgent splits the raw document into overlapping text chunks.
type ChunkAgent struct {
	ChunkSize int // characters per chunk
	Overlap   int // character overlap between consecutive chunks
}

func (a *ChunkAgent) Name() string        { return "chunk" }
func (a *ChunkAgent) Description() string { return "Splits raw document into overlapping text chunks." }
func (a *ChunkAgent) Capabilities() core.AgentCapabilities { return core.AgentCapabilities{} }

func (a *ChunkAgent) Run(ctx *core.Context) error {
	raw, _ := ctx.State["raw_document"].(string)

	size := a.ChunkSize
	if size == 0 {
		size = 512
	}
	overlap := a.Overlap
	if overlap == 0 {
		overlap = 64
	}

	var chunks []string
	text := strings.TrimSpace(raw)
	for start := 0; start < len(text); start += size - overlap {
		end := start + size
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, text[start:end])
		if end == len(text) {
			break
		}
	}

	ctx.State["chunks"] = chunks
	observability.PublishThought(ctx.GraphName, a.Name(), ctx.SessionID, fmt.Sprintf("Split document into %d chunks (size=%d, overlap=%d).", len(chunks), size, overlap))
	ctx.Logger.Info("chunk complete", "count", len(chunks))
	return nil
}
