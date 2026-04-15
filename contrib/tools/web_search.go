package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go_agent_framework/core"
)

// WebSearch simulates a web search.
type WebSearch struct{}

func (w *WebSearch) Name() string        { return "search_web" }
func (w *WebSearch) Description() string { return "Search the web for a query and return top results." }
func (w *WebSearch) Parameters() []core.ToolParameter {
	return []core.ToolParameter{
		{Name: "query", Type: "string", Description: "The search query", Required: true},
	}
}

func (w *WebSearch) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("web_search: %w", err)
	}
	// Mock response.
	return fmt.Sprintf(`[{"title":"Result for %s","url":"https://example.com","snippet":"Relevant info about %s."}]`, p.Query, p.Query), nil
}
