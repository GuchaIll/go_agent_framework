package observability

import (
	"encoding/json"
	"net/http"
)

// NodeInfo describes a single agent node in the graph.
type NodeInfo struct {
	ID          string   `json:"id"`
	Description string   `json:"description,omitempty"`
	Tools       []string `json:"tools,omitempty"`
	Skills      []string `json:"skills,omitempty"`
	RAG         []string `json:"rag,omitempty"`
	Agents      []string `json:"agents,omitempty"`
	Model       string   `json:"model,omitempty"`
}

// EdgeInfo describes a connection between two agent nodes.
type EdgeInfo struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Parallel bool   `json:"parallel"`
}

// GraphInfo is the complete structure of the agent graph.
type GraphInfo struct {
	Name  string     `json:"name"`
	Nodes []NodeInfo `json:"nodes"`
	Edges []EdgeInfo `json:"edges"`
}

// GraphInfoProvider is implemented by the application to supply graph structure.
type GraphInfoProvider interface {
	GraphInfo() GraphInfo
}

// GraphInfoHandler returns an HTTP handler that serves graph structure as JSON.
func GraphInfoHandler(provider GraphInfoProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(provider.GraphInfo())
	}
}
