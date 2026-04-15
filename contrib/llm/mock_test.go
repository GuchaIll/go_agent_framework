package llm

import (
	"context"
	"errors"
	"testing"
)

func TestMockLLMGenerateReturnsFixedResponse(t *testing.T) {
	client := &MockLLM{Response: "hello"}

	got, err := client.Generate(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got != "hello" {
		t.Fatalf("expected response hello, got %q", got)
	}
}

func TestMockLLMGenerateReturnsError(t *testing.T) {
	sentinel := errors.New("llm unavailable")
	client := &MockLLM{Response: "ignored", Err: sentinel}

	got, err := client.Generate(context.Background(), "prompt")
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected error %v, got %v", sentinel, err)
	}
	if got != "ignored" {
		t.Fatalf("expected response passthrough, got %q", got)
	}
}
