package core

// AgentCapabilities describes what resources an agent uses.
type AgentCapabilities struct {
	Tools  []string `json:"tools,omitempty"`
	Skills []string `json:"skills,omitempty"`
	RAG    []string `json:"rag,omitempty"`
	Agents []string `json:"agents,omitempty"`
	Model  string   `json:"model,omitempty"` // model role: "analysis" or "orchestration"
}

// Describable is an optional interface agents can implement to expose
// metadata for the dashboard. Agents that do not implement this will
// display with their name only.
type Describable interface {
	Description() string
	Capabilities() AgentCapabilities
}
