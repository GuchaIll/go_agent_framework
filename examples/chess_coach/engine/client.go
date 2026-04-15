package engine

import "context"

// EngineClient abstracts the chess engine (e.g. Stockfish via gRPC).
type EngineClient interface {
	ValidateFEN(ctx context.Context, fen string) (bool, error)
	Analyze(ctx context.Context, fen string, depth int) (map[string]interface{}, error)
	IsMoveLegal(ctx context.Context, fen string, move string) (bool, error)
}

// MockEngine implements EngineClient with canned responses for local dev.
type MockEngine struct{}

func (m *MockEngine) ValidateFEN(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (m *MockEngine) Analyze(_ context.Context, _ string, _ int) (map[string]interface{}, error) {
	return map[string]interface{}{
		"eval":      "+0.35",
		"best_move": "e2e4",
		"depth":     20,
		"pv":        "e2e4 e7e5 g1f3 b8c6",
	}, nil
}

func (m *MockEngine) IsMoveLegal(_ context.Context, _ string, _ string) (bool, error) {
	return true, nil
}
