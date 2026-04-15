package observability

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"go_agent_framework/contrib/llm"
)

func TestInstrumentLLMRecordsMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	client := InstrumentLLMWithMetrics(&llm.MockLLM{
		Response:     "synthetic completion",
		ProviderName: "openai",
		ModelName:    "gpt-test",
	}, metrics, LLMOptions{
		InputCostPer1KTokens:  0.5,
		OutputCostPer1KTokens: 1.5,
	})

	ctx := WithAgent(WithGraph(context.Background(), "doc_qa"), "answer")
	resp, err := client.Generate(ctx, "hello world")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if resp != "synthetic completion" {
		t.Fatalf("Generate() response = %q, want synthetic completion", resp)
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	assertMetricValue(t, families, "go_agent_framework_llm_requests_total", map[string]string{
		"graph":    "doc_qa",
		"agent":    "answer",
		"provider": "openai",
		"model":    "gpt-test",
		"status":   "success",
	}, 1)
	assertMetricValue(t, families, "go_agent_framework_llm_estimated_tokens_total", map[string]string{
		"graph":     "doc_qa",
		"agent":     "answer",
		"provider":  "openai",
		"model":     "gpt-test",
		"direction": "total",
	}, float64(estimateTokens(len("hello world"))+estimateTokens(len("synthetic completion"))))
	assertMetricValue(t, families, "go_agent_framework_llm_estimated_cost_usd_total", map[string]string{
		"graph":    "doc_qa",
		"agent":    "answer",
		"provider": "openai",
		"model":    "gpt-test",
	}, estimateCostUSD(
		estimateTokens(len("hello world")),
		estimateTokens(len("synthetic completion")),
		LLMOptions{InputCostPer1KTokens: 0.5, OutputCostPer1KTokens: 1.5},
	))
}

func assertMetricValue(t *testing.T, families []*dto.MetricFamily, name string, labels map[string]string, want float64) {
	t.Helper()

	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if matchesLabels(metric.GetLabel(), labels) {
				got := metricValue(metric)
				if got != want {
					t.Fatalf("%s labels=%v got %v want %v", name, labels, got, want)
				}
				return
			}
		}
	}

	t.Fatalf("metric %s with labels %v not found", name, labels)
}

func matchesLabels(pairs []*dto.LabelPair, want map[string]string) bool {
	if len(pairs) != len(want) {
		return false
	}
	for _, pair := range pairs {
		if want[pair.GetName()] != pair.GetValue() {
			return false
		}
	}
	return true
}

func metricValue(metric *dto.Metric) float64 {
	switch {
	case metric.GetCounter() != nil:
		return metric.GetCounter().GetValue()
	case metric.GetGauge() != nil:
		return metric.GetGauge().GetValue()
	case metric.GetHistogram() != nil:
		return float64(metric.GetHistogram().GetSampleCount())
	default:
		return 0
	}
}
