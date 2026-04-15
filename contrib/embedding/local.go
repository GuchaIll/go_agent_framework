package embedding

import (
	"context"
	"math"
)

// LocalEmbedder is a stub for a local embedding model (e.g. ONNX runtime).
// In production this would load a model; here it generates deterministic
// toy vectors for testing.
type LocalEmbedder struct {
	Dim int
}

func NewLocalEmbedder(dim int) *LocalEmbedder {
	return &LocalEmbedder{Dim: dim}
}

func (e *LocalEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return hashVector(text, e.Dim), nil
}

func (e *LocalEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = hashVector(t, e.Dim)
	}
	return out, nil
}

// hashVector produces a deterministic unit vector from a string.
func hashVector(s string, dim int) []float32 {
	v := make([]float32, dim)
	for i, ch := range s {
		v[i%dim] += float32(ch)
	}
	// Normalise to unit length.
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	if norm > 0 {
		for i := range v {
			v[i] = float32(float64(v[i]) / norm)
		}
	}
	return v
}
