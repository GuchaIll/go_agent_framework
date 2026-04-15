package vector

import (
	"context"
	"testing"
)

func TestInMemoryVectorDBSearchRoundTripAndOrdering(t *testing.T) {
	db := NewInMemoryVectorDB()
	docs := []Document{
		{ID: "a", Content: "alpha", Vector: []float32{1, 0}},
		{ID: "b", Content: "beta", Vector: []float32{0.5, 0.5}},
		{ID: "c", Content: "gamma", Vector: []float32{0, 1}},
	}

	if err := db.Insert(context.Background(), docs); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	results, err := db.Search(context.Background(), []float32{1, 0}, 0)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Document.ID != "a" || results[1].Document.ID != "b" || results[2].Document.ID != "c" {
		t.Fatalf("unexpected search ordering: %+v", results)
	}
	if results[0].Score < results[1].Score || results[1].Score < results[2].Score {
		t.Fatalf("expected descending scores, got %+v", results)
	}
}

func TestInMemoryVectorDBTopKHandling(t *testing.T) {
	db := NewInMemoryVectorDB()
	if err := db.Insert(context.Background(), []Document{
		{ID: "a", Vector: []float32{1, 0}},
		{ID: "b", Vector: []float32{0, 1}},
	}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	topOne, err := db.Search(context.Background(), []float32{1, 0}, 1)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(topOne) != 1 || topOne[0].Document.ID != "a" {
		t.Fatalf("expected top result a, got %+v", topOne)
	}

	all, err := db.Search(context.Background(), []float32{1, 0}, 0)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected all results for topK=0, got %d", len(all))
	}

	all, err = db.Search(context.Background(), []float32{1, 0}, -1)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected all results for negative topK, got %d", len(all))
	}
}

func TestInMemoryVectorDBSearchEdgeCases(t *testing.T) {
	t.Run("empty db", func(t *testing.T) {
		db := NewInMemoryVectorDB()

		results, err := db.Search(context.Background(), []float32{1, 0}, 5)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected no results, got %+v", results)
		}
	})

	t.Run("mismatched vector lengths yield zero score", func(t *testing.T) {
		db := NewInMemoryVectorDB()
		if err := db.Insert(context.Background(), []Document{
			{ID: "bad", Vector: []float32{1, 0, 0}},
		}); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		results, err := db.Search(context.Background(), []float32{1, 0}, 1)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if len(results) != 1 || results[0].Score != 0 {
			t.Fatalf("expected zero-score mismatched result, got %+v", results)
		}
	})

	t.Run("zero vector yields zero score", func(t *testing.T) {
		db := NewInMemoryVectorDB()
		if err := db.Insert(context.Background(), []Document{
			{ID: "zero", Vector: []float32{0, 0}},
		}); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}

		results, err := db.Search(context.Background(), []float32{1, 0}, 1)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if len(results) != 1 || results[0].Score != 0 {
			t.Fatalf("expected zero score for zero vector, got %+v", results)
		}
	})
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a    []float32
		b    []float32
		want float32
	}{
		{name: "identical", a: []float32{1, 0}, b: []float32{1, 0}, want: 1},
		{name: "orthogonal", a: []float32{1, 0}, b: []float32{0, 1}, want: 0},
		{name: "mismatched", a: []float32{1}, b: []float32{1, 0}, want: 0},
		{name: "empty", a: []float32{}, b: []float32{}, want: 0},
		{name: "zero vector", a: []float32{0, 0}, b: []float32{1, 0}, want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := cosineSimilarity(tc.a, tc.b); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
