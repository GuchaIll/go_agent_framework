package observability

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics groups the Prometheus collectors used by the framework.
type Metrics struct {
	graphRunsTotal       *prometheus.CounterVec
	graphDurationSeconds *prometheus.HistogramVec
	graphActiveRuns      *prometheus.GaugeVec

	agentRunsTotal       *prometheus.CounterVec
	agentDurationSeconds *prometheus.HistogramVec

	llmRequestsTotal         *prometheus.CounterVec
	llmRequestDuration       *prometheus.HistogramVec
	llmInFlight              *prometheus.GaugeVec
	llmPromptCharactersTotal *prometheus.CounterVec
	llmCompletionCharsTotal  *prometheus.CounterVec
	llmEstimatedTokensTotal  *prometheus.CounterVec
	llmEstimatedCostTotal    *prometheus.CounterVec
}

var (
	defaultMetrics     *Metrics
	defaultMetricsOnce sync.Once
)

// Default returns the process-wide metrics registry/collectors.
func Default() *Metrics {
	defaultMetricsOnce.Do(func() {
		defaultMetrics = NewMetrics(prometheus.DefaultRegisterer)
	})
	return defaultMetrics
}

// NewMetrics registers and returns a metrics bundle.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	m := &Metrics{
		graphRunsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "go_agent_framework_graph_runs_total",
				Help: "Total graph executions grouped by graph and outcome.",
			},
			[]string{"graph", "status"},
		),
		graphDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "go_agent_framework_graph_duration_seconds",
				Help:    "Graph execution latency grouped by graph and outcome.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"graph", "status"},
		),
		graphActiveRuns: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "go_agent_framework_graph_active_runs",
				Help: "Current in-flight graph executions.",
			},
			[]string{"graph"},
		),
		agentRunsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "go_agent_framework_agent_runs_total",
				Help: "Total agent executions grouped by graph, agent, and outcome.",
			},
			[]string{"graph", "agent", "status"},
		),
		agentDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "go_agent_framework_agent_duration_seconds",
				Help:    "Agent execution latency grouped by graph, agent, and outcome.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"graph", "agent", "status"},
		),
		llmRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "go_agent_framework_llm_requests_total",
				Help: "Total LLM calls grouped by graph, agent, provider, model, and outcome.",
			},
			[]string{"graph", "agent", "provider", "model", "status"},
		),
		llmRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "go_agent_framework_llm_request_duration_seconds",
				Help:    "LLM request latency grouped by graph, agent, provider, and model.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"graph", "agent", "provider", "model"},
		),
		llmInFlight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "go_agent_framework_llm_in_flight_requests",
				Help: "Current in-flight LLM requests grouped by graph, agent, provider, and model.",
			},
			[]string{"graph", "agent", "provider", "model"},
		),
		llmPromptCharactersTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "go_agent_framework_llm_prompt_characters_total",
				Help: "Total prompt characters sent to LLMs.",
			},
			[]string{"graph", "agent", "provider", "model"},
		),
		llmCompletionCharsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "go_agent_framework_llm_completion_characters_total",
				Help: "Total completion characters returned from LLMs.",
			},
			[]string{"graph", "agent", "provider", "model"},
		),
		llmEstimatedTokensTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "go_agent_framework_llm_estimated_tokens_total",
				Help: "Estimated token volume derived from character counts.",
			},
			[]string{"graph", "agent", "provider", "model", "direction"},
		),
		llmEstimatedCostTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "go_agent_framework_llm_estimated_cost_usd_total",
				Help: "Estimated LLM cost in USD using configured per-1K token prices.",
			},
			[]string{"graph", "agent", "provider", "model"},
		),
	}

	collectors := []prometheus.Collector{
		m.graphRunsTotal,
		m.graphDurationSeconds,
		m.graphActiveRuns,
		m.agentRunsTotal,
		m.agentDurationSeconds,
		m.llmRequestsTotal,
		m.llmRequestDuration,
		m.llmInFlight,
		m.llmPromptCharactersTotal,
		m.llmCompletionCharsTotal,
		m.llmEstimatedTokensTotal,
		m.llmEstimatedCostTotal,
	}
	for _, collector := range collectors {
		reg.MustRegister(collector)
	}

	return m
}

// Handler exposes the registered Prometheus metrics using the default registry.
func Handler() http.Handler {
	Default()
	return promhttp.Handler()
}

// ObserveGraphRun records graph-level execution metrics.
func (m *Metrics) ObserveGraphRun(graph string, duration time.Duration, err error) {
	status := statusLabel(err)
	m.graphRunsTotal.WithLabelValues(graph, status).Inc()
	m.graphDurationSeconds.WithLabelValues(graph, status).Observe(duration.Seconds())
}

// IncGraphInFlight increments the in-flight graph counter for graph.
func (m *Metrics) IncGraphInFlight(graph string) {
	m.graphActiveRuns.WithLabelValues(graph).Inc()
}

// DecGraphInFlight decrements the in-flight graph counter for graph.
func (m *Metrics) DecGraphInFlight(graph string) {
	m.graphActiveRuns.WithLabelValues(graph).Dec()
}

// ObserveAgentRun records agent-level execution metrics.
func (m *Metrics) ObserveAgentRun(graph, agent string, duration time.Duration, err error) {
	status := statusLabel(err)
	m.agentRunsTotal.WithLabelValues(graph, agent, status).Inc()
	m.agentDurationSeconds.WithLabelValues(graph, agent, status).Observe(duration.Seconds())
}

// ObserveLLMCall records one LLM invocation.
func (m *Metrics) ObserveLLMCall(labels LLMLabels, duration time.Duration, promptChars, completionChars, promptTokens, completionTokens int, costUSD float64, err error) {
	status := statusLabel(err)

	m.llmRequestsTotal.WithLabelValues(labels.Graph, labels.Agent, labels.Provider, labels.Model, status).Inc()
	m.llmRequestDuration.WithLabelValues(labels.Graph, labels.Agent, labels.Provider, labels.Model).Observe(duration.Seconds())

	if promptChars > 0 {
		m.llmPromptCharactersTotal.WithLabelValues(labels.Graph, labels.Agent, labels.Provider, labels.Model).Add(float64(promptChars))
	}
	if completionChars > 0 {
		m.llmCompletionCharsTotal.WithLabelValues(labels.Graph, labels.Agent, labels.Provider, labels.Model).Add(float64(completionChars))
	}
	if promptTokens > 0 {
		m.llmEstimatedTokensTotal.WithLabelValues(labels.Graph, labels.Agent, labels.Provider, labels.Model, "prompt").Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		m.llmEstimatedTokensTotal.WithLabelValues(labels.Graph, labels.Agent, labels.Provider, labels.Model, "completion").Add(float64(completionTokens))
	}
	totalTokens := promptTokens + completionTokens
	if totalTokens > 0 {
		m.llmEstimatedTokensTotal.WithLabelValues(labels.Graph, labels.Agent, labels.Provider, labels.Model, "total").Add(float64(totalTokens))
	}
	if costUSD > 0 {
		m.llmEstimatedCostTotal.WithLabelValues(labels.Graph, labels.Agent, labels.Provider, labels.Model).Add(costUSD)
	}
}

// IncLLMInFlight increments the in-flight LLM request count for labels.
func (m *Metrics) IncLLMInFlight(labels LLMLabels) {
	m.llmInFlight.WithLabelValues(labels.Graph, labels.Agent, labels.Provider, labels.Model).Inc()
}

// DecLLMInFlight decrements the in-flight LLM request count for labels.
func (m *Metrics) DecLLMInFlight(labels LLMLabels) {
	m.llmInFlight.WithLabelValues(labels.Graph, labels.Agent, labels.Provider, labels.Model).Dec()
}

func statusLabel(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}
