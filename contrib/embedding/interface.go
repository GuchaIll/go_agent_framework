package embedding

import "context"

// Embedder produces vector embeddings from text.
type Embedder interface {
	// Embed returns a single embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch returns embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}
