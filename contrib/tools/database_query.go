package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go_agent_framework/core"
)

// DatabaseQuery simulates a database query tool.
type DatabaseQuery struct{}

func (d *DatabaseQuery) Name() string { return "query_db" }
func (d *DatabaseQuery) Description() string {
	return "Run a read-only SQL query against the application database."
}
func (d *DatabaseQuery) Parameters() []core.ToolParameter {
	return []core.ToolParameter{
		{Name: "query", Type: "string", Description: "SQL SELECT query", Required: true},
	}
}

func (d *DatabaseQuery) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("database_query: %w", err)
	}
	// Mock response.
	return fmt.Sprintf(`{"rows":[{"id":1,"name":"example"}],"query":%q}`, p.Query), nil
}
