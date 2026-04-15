package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"go_agent_framework/contrib/embedding"
	"go_agent_framework/contrib/envutil"
	"go_agent_framework/contrib/llm"
	contribtools "go_agent_framework/contrib/tools"
	"go_agent_framework/contrib/vector"
	"go_agent_framework/core"
	multiagent "go_agent_framework/examples/multi_agent_tools"
	"go_agent_framework/observability"
)

func main() {
	envutil.Load(".env")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	models := buildModels(logger)

	// --- Tool registry ---
	toolReg := core.NewToolRegistry()
	if err := contribtools.RegisterAll(toolReg); err != nil {
		log.Fatal(err)
	}

	// --- Vector DB + embedding for RAG ---
	embedder := embedding.NewLocalEmbedder(64)
	vdb := vector.NewInMemoryVectorDB()
	seedDocs(vdb, embedder)
	retriever := &embeddingRetriever{db: vdb, embedder: embedder}

	// --- Build graph + orchestrator ---
	graph := multiagent.BuildToolSkillGraph(models, toolReg, retriever)
	store := core.NewMemStore()
	orch := core.NewOrchestrator(graph, store, 8)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /chat", orch.HandleRequest)
	mux.Handle("/metrics", observability.Handler())

	// Dashboard with chat
	adapter := &multiagent.GraphAdapter{Graph: graph}
	chatHandler := makeChatHandler(graph, store)
	dashMux := observability.DashboardMux(adapter, os.DirFS("dashboard/dist"), chatHandler)
	mux.Handle("/dashboard/", dashMux)

	addr := ":8082"
	logger.Info("multi-agent-tools listening", "addr", addr)
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

		session.State["user_query"] = body.Message

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

// embeddingRetriever bridges the Embedder + VectorDB into core.Retriever.
type embeddingRetriever struct {
	db       *vector.InMemoryVectorDB
	embedder *embedding.LocalEmbedder
}

func (r *embeddingRetriever) Retrieve(ctx context.Context, query string, topK int) ([]core.Chunk, error) {
	vec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	results, err := r.db.Search(ctx, vec, topK)
	if err != nil {
		return nil, err
	}
	chunks := make([]core.Chunk, len(results))
	for i, sr := range results {
		chunks[i] = core.Chunk{
			ID:      sr.Document.ID,
			Content: sr.Document.Content,
			Score:   sr.Score,
		}
	}
	return chunks, nil
}

func seedDocs(db *vector.InMemoryVectorDB, embedder *embedding.LocalEmbedder) {
	docs := []string{
		"Go is a statically typed, compiled language designed at Google.",
		"Goroutines are lightweight threads managed by the Go runtime.",
		"Channels are the pipes that connect concurrent goroutines.",
	}
	var vdocs []vector.Document
	for i, d := range docs {
		vec, _ := embedder.Embed(context.Background(), d)
		vdocs = append(vdocs, vector.Document{
			ID:      fmt.Sprintf("doc_%d", i),
			Content: d,
			Vector:  vec,
		})
	}
	_ = db.Insert(context.Background(), vdocs)
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
		Response:     `[{"type":"tool","name":"get_weather","args":{"location":"London"}}]`,
		ProviderName: "mock",
		ModelName:    "multi-agent-demo",
	}, observability.LLMOptions{})
	return llm.Models{Analysis: mock, Orchestration: mock}
}
