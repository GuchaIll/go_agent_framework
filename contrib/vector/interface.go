package vector

import "context"

// Document represents a stored chunk with its embedding vector.
type Document struct {
	ID      string
	Content string
	Vector  []float32
}

// SearchResult is a document with a similarity score.
type SearchResult struct {
	Document Document
	Score    float32
}

// VectorDB is the interface for vector storage and similarity search.
type VectorDB interface {
	Insert(ctx context.Context, docs []Document) error
	Search(ctx context.Context, queryVector []float32, topK int) ([]SearchResult, error)
}
