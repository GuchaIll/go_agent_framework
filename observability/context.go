package observability

import "context"

type contextKey string

const (
	graphKey   contextKey = "go_agent_framework.graph"
	agentKey   contextKey = "go_agent_framework.agent"
	sessionKey contextKey = "go_agent_framework.session"
)

// WithGraph annotates a context with the current graph name.
func WithGraph(ctx context.Context, graph string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if graph == "" {
		return ctx
	}
	return context.WithValue(ctx, graphKey, graph)
}

// WithAgent annotates a context with the current agent name.
func WithAgent(ctx context.Context, agent string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if agent == "" {
		return ctx
	}
	return context.WithValue(ctx, agentKey, agent)
}

// WithSession annotates a context with the current session identifier.
func WithSession(ctx context.Context, sessionID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, sessionKey, sessionID)
}

// GraphFromContext returns the graph label stored in ctx.
func GraphFromContext(ctx context.Context) string {
	return stringValue(ctx, graphKey, "unknown_graph")
}

// AgentFromContext returns the agent label stored in ctx.
func AgentFromContext(ctx context.Context) string {
	return stringValue(ctx, agentKey, "unknown_agent")
}

// SessionFromContext returns the session label stored in ctx.
func SessionFromContext(ctx context.Context) string {
	return stringValue(ctx, sessionKey, "")
}

func stringValue(ctx context.Context, key contextKey, fallback string) string {
	if ctx == nil {
		return fallback
	}
	if v, ok := ctx.Value(key).(string); ok && v != "" {
		return v
	}
	return fallback
}
