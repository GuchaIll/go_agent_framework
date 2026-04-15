package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go_agent_framework/core"
)

// Calculator evaluates simple arithmetic expressions.
type Calculator struct{}

func (c *Calculator) Name() string { return "calculate" }
func (c *Calculator) Description() string {
	return "Evaluate a simple arithmetic expression (+, -, *, /)."
}
func (c *Calculator) Parameters() []core.ToolParameter {
	return []core.ToolParameter{
		{Name: "a", Type: "number", Description: "First operand", Required: true},
		{Name: "b", Type: "number", Description: "Second operand", Required: true},
		{Name: "op", Type: "string", Description: "Operator: +, -, *, /", Required: true},
	}
}

func (c *Calculator) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var p struct {
		A  float64 `json:"a"`
		B  float64 `json:"b"`
		Op string  `json:"op"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("calculator: %w", err)
	}

	var result float64
	switch p.Op {
	case "+":
		result = p.A + p.B
	case "-":
		result = p.A - p.B
	case "*":
		result = p.A * p.B
	case "/":
		if p.B == 0 {
			return "", fmt.Errorf("calculator: division by zero")
		}
		result = p.A / p.B
	default:
		return "", fmt.Errorf("calculator: unknown operator %q", p.Op)
	}
	return fmt.Sprintf("%g", result), nil
}
