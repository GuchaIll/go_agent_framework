package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const openRouterBaseURL = "https://openrouter.ai/api/v1/chat/completions"

// OpenRouterClient calls the OpenRouter chat-completions API.
// It implements both LLMClient and Describer.
type OpenRouterClient struct {
	APIKey     string
	ModelName  string       // e.g. "z-ai/glm-4.5-air:free"
	BaseURL    string       // override for testing; defaults to openRouterBaseURL
	HTTPClient *http.Client // defaults to http.DefaultClient
}

// --- OpenAI-compatible request/response types ---

type orMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type orRequest struct {
	Model    string      `json:"model"`
	Messages []orMessage `json:"messages"`
}

type orChoice struct {
	Message orMessage `json:"message"`
}

type orErrorDetail struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type orResponse struct {
	Choices []orChoice     `json:"choices"`
	Error   *orErrorDetail `json:"error,omitempty"`
}

// Generate sends a single user-message prompt to OpenRouter and returns the
// assistant's reply.
func (c *OpenRouterClient) Generate(ctx context.Context, prompt string) (string, error) {
	endpoint := c.BaseURL
	if endpoint == "" {
		endpoint = openRouterBaseURL
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	payload := orRequest{
		Model:    c.ModelName,
		Messages: []orMessage{{Role: "user", Content: prompt}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("openrouter: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("openrouter: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter: request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("openrouter: read body: %w", err)
	}

	var orResp orResponse
	if err := json.Unmarshal(raw, &orResp); err != nil {
		return "", fmt.Errorf("openrouter: decode: %w (status %d, body: %.256s)", err, resp.StatusCode, string(raw))
	}

	if orResp.Error != nil {
		return "", fmt.Errorf("openrouter: API error %d: %s", orResp.Error.Code, orResp.Error.Message)
	}

	if len(orResp.Choices) == 0 {
		return "", fmt.Errorf("openrouter: empty choices (status %d)", resp.StatusCode)
	}

	return orResp.Choices[0].Message.Content, nil
}

// Provider implements llm.Describer.
func (c *OpenRouterClient) Provider() string { return "openrouter" }

// Model implements llm.Describer.
func (c *OpenRouterClient) Model() string { return c.ModelName }
