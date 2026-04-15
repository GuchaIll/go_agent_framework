package chesstools

import (
	"context"
	"encoding/json"
	"fmt"
	"go_agent_framework/core"
	"go_agent_framework/examples/chess_coach/engine"
)

// ValidateFENTool validates a FEN string using the chess engine.
type ValidateFENTool struct {
	Engine engine.EngineClient
}

func (t *ValidateFENTool) Name() string        { return "validate_fen" }
func (t *ValidateFENTool) Description() string { return "Validate a FEN position string using the chess engine." }
func (t *ValidateFENTool) Parameters() []core.ToolParameter {
	return []core.ToolParameter{
		{Name: "fen", Type: "string", Description: "FEN position string to validate", Required: true},
	}
}

func (t *ValidateFENTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		FEN string `json:"fen"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("validate_fen: %w", err)
	}
	valid, err := t.Engine.ValidateFEN(ctx, p.FEN)
	if err != nil {
		return "", fmt.Errorf("validate_fen: %w", err)
	}
	out, _ := json.Marshal(map[string]bool{"valid": valid})
	return string(out), nil
}

// AnalyzePositionTool runs engine analysis on a FEN position.
type AnalyzePositionTool struct {
	Engine engine.EngineClient
}

func (t *AnalyzePositionTool) Name() string { return "analyze_position" }
func (t *AnalyzePositionTool) Description() string {
	return "Run engine analysis on a FEN position at a given depth."
}
func (t *AnalyzePositionTool) Parameters() []core.ToolParameter {
	return []core.ToolParameter{
		{Name: "fen", Type: "string", Description: "FEN position string to analyze", Required: true},
		{Name: "depth", Type: "number", Description: "Search depth for engine analysis", Required: true},
	}
}

func (t *AnalyzePositionTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		FEN   string `json:"fen"`
		Depth int    `json:"depth"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("analyze_position: %w", err)
	}
	if p.Depth <= 0 {
		p.Depth = 20
	}
	metrics, err := t.Engine.Analyze(ctx, p.FEN, p.Depth)
	if err != nil {
		return "", fmt.Errorf("analyze_position: %w", err)
	}
	out, _ := json.Marshal(metrics)
	return string(out), nil
}

// CheckMoveTool checks if a move is legal in a given position.
type CheckMoveTool struct {
	Engine engine.EngineClient
}

func (t *CheckMoveTool) Name() string        { return "is_move_legal" }
func (t *CheckMoveTool) Description() string { return "Check whether a proposed move is legal in the given FEN position." }
func (t *CheckMoveTool) Parameters() []core.ToolParameter {
	return []core.ToolParameter{
		{Name: "fen", Type: "string", Description: "FEN position string", Required: true},
		{Name: "move", Type: "string", Description: "Move in UCI notation (e.g. e2e4)", Required: true},
	}
}

func (t *CheckMoveTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p struct {
		FEN  string `json:"fen"`
		Move string `json:"move"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("is_move_legal: %w", err)
	}
	legal, err := t.Engine.IsMoveLegal(ctx, p.FEN, p.Move)
	if err != nil {
		return "", fmt.Errorf("is_move_legal: %w", err)
	}
	out, _ := json.Marshal(map[string]bool{"legal": legal})
	return string(out), nil
}

// ValidateEngineTool is a mock tool that validates the engine is operational
// by running a quick analysis on the starting position.
type ValidateEngineTool struct {
	Engine engine.EngineClient
}

func (t *ValidateEngineTool) Name() string { return "validate_engine" }
func (t *ValidateEngineTool) Description() string {
	return "Validate that the chess engine is operational by running a quick analysis."
}
func (t *ValidateEngineTool) Parameters() []core.ToolParameter { return nil }

func (t *ValidateEngineTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	const startFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	metrics, err := t.Engine.Analyze(ctx, startFEN, 1)
	if err != nil {
		return "", fmt.Errorf("validate_engine: %w", err)
	}
	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"metrics": metrics,
	})
	return string(out), nil
}

// RegisterChessTools registers all chess engine tools with the given registry.
func RegisterChessTools(reg *core.ToolRegistry, eng engine.EngineClient) error {
	for _, t := range []core.Tool{
		&ValidateFENTool{Engine: eng},
		&AnalyzePositionTool{Engine: eng},
		&CheckMoveTool{Engine: eng},
		&ValidateEngineTool{Engine: eng},
	} {
		if err := reg.Register(t); err != nil {
			return err
		}
	}
	return nil
}
