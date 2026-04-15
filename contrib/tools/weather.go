package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go_agent_framework/core"
)

// Weather returns mock weather data for a location.
type Weather struct{}

func (w *Weather) Name() string        { return "get_weather" }
func (w *Weather) Description() string { return "Get the current weather for a location." }
func (w *Weather) Parameters() []core.ToolParameter {
	return []core.ToolParameter{
		{Name: "location", Type: "string", Description: "City name or coordinates", Required: true},
	}
}

func (w *Weather) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Location string `json:"location"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("weather: %w", err)
	}
	// Mock response.
	return fmt.Sprintf(`{"location":%q,"temp_c":22,"condition":"Sunny"}`, p.Location), nil
}
