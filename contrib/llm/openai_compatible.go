package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const openAIChatCompletionsURL = "https://api.openai.com/v1/chat/completions"

// OpenAICompatibleClient calls an OpenAI-style chat completions endpoint.
// This works with OpenAI itself as well as self-hosted vLLM servers that expose
// /v1/chat/completions-compatible APIs.
type OpenAICompatibleClient struct {
	APIKey       string
	ModelName    string
	BaseURL      string
	ProviderName string
	HTTPClient   *http.Client
}

type oaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaRequest struct {
	Model    string      `json:"model"`
	Messages []oaMessage `json:"messages"`
}

type oaChoice struct {
	Message struct {
		Content interface{} `json:"content"`
	} `json:"message"`
}

type oaErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Code    any    `json:"code,omitempty"`
}

type oaResponse struct {
	Choices []oaChoice     `json:"choices"`
	Error   *oaErrorDetail `json:"error,omitempty"`
}

func (c *OpenAICompatibleClient) Generate(ctx context.Context, prompt string) (string, error) {
	endpoint := c.endpoint()
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	payload := oaRequest{
		Model:    c.ModelName,
		Messages: []oaMessage{{Role: "user", Content: prompt}},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("%s: marshal: %w", c.Provider(), err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%s: new request: %w", c.Provider(), err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: request: %w", c.Provider(), err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%s: read body: %w", c.Provider(), err)
	}

	var parsed oaResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("%s: decode: %w (status %d, body: %.256s)", c.Provider(), err, resp.StatusCode, string(raw))
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("%s: API error: %s", c.Provider(), parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("%s: empty choices (status %d)", c.Provider(), resp.StatusCode)
	}

	content := renderOpenAIContent(parsed.Choices[0].Message.Content)
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("%s: empty completion content", c.Provider())
	}
	return content, nil
}

func (c *OpenAICompatibleClient) Provider() string {
	if strings.TrimSpace(c.ProviderName) != "" {
		return c.ProviderName
	}
	return "openai-compatible"
}

func (c *OpenAICompatibleClient) Model() string { return c.ModelName }

func (c *OpenAICompatibleClient) endpoint() string {
	base := strings.TrimSpace(c.BaseURL)
	if base == "" {
		return openAIChatCompletionsURL
	}
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

func renderOpenAIContent(raw interface{}) string {
	switch content := raw.(type) {
	case string:
		return content
	case []interface{}:
		parts := make([]string, 0, len(content))
		for _, item := range content {
			block, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			text, _ := block["text"].(string)
			if strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprint(raw)
	}
}
