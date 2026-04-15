package llm

import "context"

// LLMClient is the interface for language model backends.
type LLMClient interface {
	// Generate produces a text completion from the given prompt.
	Generate(ctx context.Context, prompt string) (string, error)
}

// Describer is an optional interface that LLM clients can implement to
// expose provider and model metadata for metrics labelling.
type Describer interface {
	Provider() string
	Model() string
}

// ModelRole indicates the intended purpose of a model in the pipeline.
type ModelRole string

const (
	// RoleAnalysis is for heavy reasoning tasks (evaluation, answer generation).
	RoleAnalysis ModelRole = "analysis"
	// RoleOrchestration is for lighter tasks (planning, tool selection, coordination).
	RoleOrchestration ModelRole = "orchestration"
)

// Models holds LLM clients keyed by their intended role.
type Models struct {
	Analysis      LLMClient // heavy reasoning model
	Orchestration LLMClient // lighter model for planning / tool selection
}

// For returns the client for the given role, falling back to Analysis.
func (m Models) For(role ModelRole) LLMClient {
	switch role {
	case RoleOrchestration:
		if m.Orchestration != nil {
			return m.Orchestration
		}
	}
	return m.Analysis
}
