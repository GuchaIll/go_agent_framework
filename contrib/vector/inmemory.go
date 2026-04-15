package vector

import (
	"context"
	"math"
	"sort"
	"sync"
)

// InMemoryVectorDB is a naive in-memory vector store using cosine similarity.
type InMemoryVectorDB struct {
	mu   sync.RWMutex
	docs []Document
}

func NewInMemoryVectorDB() *InMemoryVectorDB {
	return &InMemoryVectorDB{}
}

func (db *InMemoryVectorDB) Insert(_ context.Context, docs []Document) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.docs = append(db.docs, docs...)
	return nil
}

func (db *InMemoryVectorDB) Search(_ context.Context, queryVector []float32, topK int) ([]SearchResult, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	results := make([]SearchResult, 0, len(db.docs))
	for _, doc := range db.docs {
		score := cosineSimilarity(queryVector, doc.Vector)
		results = append(results, SearchResult{Document: doc, Score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && topK < len(results) {
		results = results[:topK]
	}
	return results, nil
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return float32(dot / denom)
}
