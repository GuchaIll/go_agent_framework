package observability

import (
	"context"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"go_agent_framework/contrib/llm"
)

// LLMOptions configures how LLM metrics are labeled and costed.
type LLMOptions struct {
	Provider              string
	Model                 string
	InputCostPer1KTokens  float64
	OutputCostPer1KTokens float64
}

// LLMLabels is the low-cardinality label set used for LLM metrics.
type LLMLabels struct {
	Graph    string
	Agent    string
	Provider string
	Model    string
}

type instrumentedLLM struct {
	base    llm.LLMClient
	metrics *Metrics
	opts    LLMOptions
}

// InstrumentLLM wraps an LLM client with Prometheus metrics using the default registry.
func InstrumentLLM(base llm.LLMClient, opts LLMOptions) llm.LLMClient {
	return InstrumentLLMWithMetrics(base, Default(), opts)
}

// InstrumentLLMWithMetrics wraps an LLM client with Prometheus metrics using a custom registry.
func InstrumentLLMWithMetrics(base llm.LLMClient, metrics *Metrics, opts LLMOptions) llm.LLMClient {
	if base == nil {
		return nil
	}
	if metrics == nil {
		metrics = Default()
	}
	return &instrumentedLLM{
		base:    base,
		metrics: metrics,
		opts:    opts,
	}
}

func (i *instrumentedLLM) Generate(ctx context.Context, prompt string) (string, error) {
	labels := i.labels(ctx)
	start := time.Now()

	i.metrics.IncLLMInFlight(labels)
	defer i.metrics.DecLLMInFlight(labels)

	completion, err := i.base.Generate(ctx, prompt)
	duration := time.Since(start)

	promptChars := utf8.RuneCountInString(prompt)
	completionChars := utf8.RuneCountInString(completion)
	promptTokens := estimateTokens(promptChars)
	completionTokens := estimateTokens(completionChars)
	costUSD := estimateCostUSD(promptTokens, completionTokens, i.opts)

	i.metrics.ObserveLLMCall(labels, duration, promptChars, completionChars, promptTokens, completionTokens, costUSD, err)
	return completion, err
}

func (i *instrumentedLLM) labels(ctx context.Context) LLMLabels {
	provider := i.opts.Provider
	model := i.opts.Model

	if descriptor, ok := i.base.(llm.Describer); ok {
		if provider == "" {
			provider = descriptor.Provider()
		}
		if model == "" {
			model = descriptor.Model()
		}
	}

	return LLMLabels{
		Graph:    GraphFromContext(ctx),
		Agent:    AgentFromContext(ctx),
		Provider: fallbackLabel(provider, "unknown_provider"),
		Model:    fallbackLabel(model, "unknown_model"),
	}
}

func fallbackLabel(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func estimateTokens(characters int) int {
	if characters <= 0 {
		return 0
	}
	return int(math.Ceil(float64(characters) / 4.0))
}

func estimateCostUSD(promptTokens, completionTokens int, opts LLMOptions) float64 {
	if promptTokens == 0 && completionTokens == 0 {
		return 0
	}
	return (float64(promptTokens) * opts.InputCostPer1KTokens / 1000.0) +
		(float64(completionTokens) * opts.OutputCostPer1KTokens / 1000.0)
}
