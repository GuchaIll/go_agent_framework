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
	"go_agent_framework/contrib/skills"
	"go_agent_framework/core"
	chess "go_agent_framework/examples/chess_coach"
	"go_agent_framework/examples/chess_coach/engine"
	chesstools "go_agent_framework/examples/chess_coach/tools"
	"go_agent_framework/observability"
)

func main() {
	envutil.Load(".env")
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	eng := &engine.MockEngine{}
	models := buildModels(logger)

	// Set up tool registry with chess engine tools.
	toolReg := core.NewToolRegistry()
	if err := chesstools.RegisterChessTools(toolReg, eng); err != nil {
		log.Fatalf("register chess tools: %v", err)
	}
	logger.Info("chess tools registered", "tools", toolReg.List())

	// Set up skill registry and load coaching skills.
	skillReg := core.NewSkillRegistry()
	if err := skills.LoadFromDir("examples/chess_coach/skills", skillReg); err != nil {
		logger.Warn("could not load skills dir, registering inline", "err", err)
		_ = skillReg.Register(&core.Skill{
			Name:        "beginner_coaching",
			Description: "Tailor your chess coaching advice towards beginners: make answers concise, explain key turns, refer to pieces by their full name in addition to color.",
			Steps:       []core.SkillStep{{Name: "format_advice", Kind: core.KindLLM}},
		})
	}
	logger.Info("skills registered", "skills", skillReg.List())

	graph := chess.BuildGraph(toolReg, skillReg, models)
	store := core.NewMemStore()
	orch := core.NewOrchestrator(graph, store, 8)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /coach", orch.HandleRequest)
	mux.Handle("/metrics", observability.Handler())

	// Dashboard with chat
	adapter := &chess.GraphAdapter{Graph: graph}
	chatHandler := makeChatHandler(graph, store)
	dashMux := observability.DashboardMux(adapter, os.DirFS("dashboard/dist"), chatHandler)
	mux.Handle("/dashboard/", dashMux)

	addr := ":8080"
	logger.Info("chess-coach listening", "addr", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
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
		Response:     "Consider controlling the centre with pawns and developing your knights early.",
		ProviderName: "mock",
		ModelName:    "chess-coach-demo",
	}, observability.LLMOptions{})
	return llm.Models{Analysis: mock, Orchestration: mock}
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

		// Map chat message to chess_coach expected input.
		// IngestAgent will extract FEN, move, and question from the raw message.
		session.State["fen"] = body.Message
		session.State["question"] = body.Message
		session.State["move"] = ""

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

		// Extract response from state.
		answer := ""
		if fb, ok := ctx.State["feedback"].(string); ok {
			answer = fb
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
