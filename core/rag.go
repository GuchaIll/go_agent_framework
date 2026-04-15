package core

import (
	"context"
	"fmt"
	"strings"
)

// Chunk is a piece of retrieved content with metadata.
type Chunk struct {
	ID      string  `json:"id"`
	Content string  `json:"content"`
	Score   float32 `json:"score"`
}

// Retriever fetches relevant chunks for a query.
type Retriever interface {
	Retrieve(ctx context.Context, query string, topK int) ([]Chunk, error)
}

// ContextInjector formats retrieved chunks into a prompt fragment.
type ContextInjector struct {
	// Header is prepended before the chunks. Default: "Context:\n"
	Header string
	// Separator between chunks. Default: "\n---\n"
	Separator string
}

// Inject formats chunks into a string suitable for inclusion in a prompt.
func (ci *ContextInjector) Inject(chunks []Chunk) string {
	header := ci.Header
	if header == "" {
		header = "Context:\n"
	}
	sep := ci.Separator
	if sep == "" {
		sep = "\n---\n"
	}
	parts := make([]string, len(chunks))
	for i, c := range chunks {
		parts[i] = c.Content
	}
	return header + strings.Join(parts, sep)
}

// RAGAgent is a graph-compatible Agent that:
//  1. Reads a query from ctx.State["rag_query"] (string).
//  2. Retrieves top-K chunks via a Retriever.
//  3. Formats them with ContextInjector and stores the result in
//     ctx.State["rag_context"] (string).
//  4. Also stores the raw chunks in ctx.State["rag_chunks"].
type RAGAgent struct {
	Retriever Retriever
	TopK      int
	Injector  *ContextInjector
}

func (a *RAGAgent) Name() string        { return "rag_retriever" }
func (a *RAGAgent) Description() string { return "Retrieves relevant context chunks from the vector store." }
func (a *RAGAgent) Capabilities() AgentCapabilities {
	return AgentCapabilities{RAG: []string{"retriever"}}
}

func (a *RAGAgent) Run(ctx *Context) error {
	query, _ := ctx.State["rag_query"].(string)
	if query == "" {
		return fmt.Errorf("rag_retriever: no rag_query in state")
	}

	topK := a.TopK
	if topK == 0 {
		topK = 5
	}

	chunks, err := a.Retriever.Retrieve(context.Background(), query, topK)
	if err != nil {
		return fmt.Errorf("rag_retriever: %w", err)
	}

	injector := a.Injector
	if injector == nil {
		injector = &ContextInjector{}
	}

	ctx.State["rag_chunks"] = chunks
	ctx.State["rag_context"] = injector.Inject(chunks)
	ctx.Logger.Info("rag retrieved", "chunks", len(chunks), "query_len", len(query))
	return nil
}
