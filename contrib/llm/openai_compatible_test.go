package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestOpenAICompatibleClientGenerate(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization header = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		text := string(body)
		if !strings.Contains(text, `"model":"Qwen/Qwen3-4B-Instruct-2507"`) {
			t.Fatalf("request missing model: %s", text)
		}
		if !strings.Contains(text, `"content":"Explain this move."`) {
			t.Fatalf("request missing prompt: %s", text)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"Centralize your rook before attacking."}}]}`)),
		}, nil
	})}

	client := &OpenAICompatibleClient{
		APIKey:       "test-key",
		ModelName:    "Qwen/Qwen3-4B-Instruct-2507",
		BaseURL:      "https://modal.example/v1",
		ProviderName: "modal",
		HTTPClient:   httpClient,
	}

	got, err := client.Generate(context.Background(), "Explain this move.")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got != "Centralize your rook before attacking." {
		t.Fatalf("Generate() response = %q", got)
	}
}

func TestOpenAICompatibleClientAppendsChatCompletionsPath(t *testing.T) {
	client := &OpenAICompatibleClient{BaseURL: "https://modal.example/v1"}
	if got := client.endpoint(); got != "https://modal.example/v1/chat/completions" {
		t.Fatalf("endpoint() = %q", got)
	}
}
