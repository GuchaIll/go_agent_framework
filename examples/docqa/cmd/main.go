package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go_agent_framework/contrib/envutil"
	"go_agent_framework/contrib/llm"
	"go_agent_framework/contrib/vector"
	"go_agent_framework/core"
	docqa "go_agent_framework/examples/docqa"
	"go_agent_framework/examples/docqa/agents"
	"go_agent_framework/observability"
)

func main() {
	envutil.Load(".env")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	embedder := &agents.MockEmbedder{Dim: 128}
	vdb := vector.NewInMemoryVectorDB()
	models := buildModels(logger)

	graph := docqa.BuildGraph(embedder, vdb, models)
	store := core.NewMemStore()
	orch := core.NewOrchestrator(graph, store, 8)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /ask", orch.HandleRequest)
	mux.Handle("/metrics", observability.Handler())

	// Dashboard with chat
	adapter := &docqa.GraphAdapter{Graph: graph}
	chatHandler := makeChatHandler(graph, store)
	dashMux := observability.DashboardMux(adapter, os.DirFS("dashboard/dist"), chatHandler)
	mux.Handle("/dashboard/", dashMux)

	addr := ":8081"
	logger.Info("docqa listening", "addr", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func makeChatHandler(graph *core.Graph, store core.StateStore) observability.ChatHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Message   string `json:"message"`
			SessionID string `json:"session_id,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if body.Message == "" {
			http.Error(w, "message is required", http.StatusBadRequest)
			return
		}

		sessionID := body.SessionID
		if sessionID == "" {
			sessionID = fmt.Sprintf("chat-%d", time.Now().UnixMilli())
		}

		session, err := store.Get(sessionID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		session.State["document"] = body.Message
		session.State["query"] = body.Message

		ctx := &core.Context{
			SessionID:  sessionID,
			State:      session.State,
			Logger:     slog.Default(),
			StdContext: r.Context(),
		}

		if err := graph.Run(ctx); err != nil {
			observability.PublishChatResponse(graph.Name(), sessionID, "Error: "+err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_ = store.Put(session)

		answer := ""
		if a, ok := ctx.State["answer"].(string); ok {
			answer = a
		}

		observability.PublishChatResponse(graph.Name(), sessionID, answer)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session_id": sessionID,
			"response":   answer,
			"state":      ctx.State,
		})
	}
}

func buildModels(logger *slog.Logger) llm.Models {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey != "" {
		logger.Info("using OpenRouter LLMs")
		analysis := observability.InstrumentLLM(
			&llm.OpenRouterClient{APIKey: apiKey, ModelName: "z-ai/glm-4.5-air:free"},
			observability.LLMOptions{},
		)
		orchestration := observability.InstrumentLLM(
			&llm.OpenRouterClient{APIKey: apiKey, ModelName: "liquid/lfm-2.5-1.2b-instruct:free"},
			observability.LLMOptions{},
		)
		return llm.Models{Analysis: analysis, Orchestration: orchestration}
	}

	logger.Info("OPENROUTER_API_KEY not set, using mock LLMs")
	mock := observability.InstrumentLLM(&llm.MockLLM{
		Response:     "Based on the provided context, the answer is 42.",
		ProviderName: "mock",
		ModelName:    "docqa-demo",
	}, observability.LLMOptions{})
	return llm.Models{Analysis: mock, Orchestration: mock}
}
