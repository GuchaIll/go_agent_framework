package observability

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	dto "github.com/prometheus/client_model/go"

	"github.com/prometheus/client_golang/prometheus"
)

// ChatHandler is a function that processes a chat message and returns the
// final state. The implementation lives in the example app and runs the
// graph. SSE events (thought, tool_call, etc.) are published by the
// agents via DefaultHub().
type ChatHandler func(w http.ResponseWriter, r *http.Request)

// DashboardMux builds an http.ServeMux with all dashboard endpoints.
// staticFS may be nil; if provided it serves the built React SPA.
// chatHandler may be nil; if provided it registers POST /dashboard/chat.
func DashboardMux(provider GraphInfoProvider, staticFS fs.FS, chatHandler ChatHandler) *http.ServeMux {
	mux := http.NewServeMux()
	hub := DefaultHub()

	mux.HandleFunc("GET /dashboard/events", hub.SSEHandler())
	mux.HandleFunc("GET /dashboard/graph", GraphInfoHandler(provider))
	mux.HandleFunc("GET /dashboard/stats", tokenStatsHandler)

	if chatHandler != nil {
		mux.HandleFunc("POST /dashboard/chat", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			chatHandler(w, r)
		})
		// CORS preflight for chat endpoint
		mux.HandleFunc("OPTIONS /dashboard/chat", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
		})
	}

	if staticFS != nil {
		fileServer := http.FileServer(http.FS(staticFS))
		mux.HandleFunc("/dashboard/", func(w http.ResponseWriter, r *http.Request) {
			// SPA fallback: serve index.html for paths without a file extension
			path := strings.TrimPrefix(r.URL.Path, "/dashboard")
			if path == "" || path == "/" {
				path = "/index.html"
			}
			// Try the real file first
			if f, err := staticFS.Open(strings.TrimPrefix(path, "/")); err == nil {
				f.Close()
				http.StripPrefix("/dashboard", fileServer).ServeHTTP(w, r)
				return
			}
			// Fall back to index.html for SPA routes
			r.URL.Path = "/dashboard/index.html"
			http.StripPrefix("/dashboard", fileServer).ServeHTTP(w, r)
		})
	}

	return mux
}

// tokenStatsHandler scrapes the default Prometheus gatherer for LLM metrics
// and returns them as a simple JSON object.
func tokenStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}

	stats := map[string]float64{
		"llm_requests_total":         0,
		"llm_estimated_tokens_total": 0,
		"llm_prompt_tokens":          0,
		"llm_completion_tokens":      0,
		"llm_estimated_cost_usd":     0,
	}

	for _, mf := range families {
		name := mf.GetName()
		switch name {
		case "go_agent_framework_llm_requests_total":
			stats["llm_requests_total"] = sumCounter(mf)
		case "go_agent_framework_llm_estimated_tokens_total":
			for _, m := range mf.GetMetric() {
				dir := labelValue(m, "direction")
				val := m.GetCounter().GetValue()
				switch dir {
				case "prompt":
					stats["llm_prompt_tokens"] += val
				case "completion":
					stats["llm_completion_tokens"] += val
				case "total":
					stats["llm_estimated_tokens_total"] += val
				}
			}
		case "go_agent_framework_llm_estimated_cost_usd_total":
			stats["llm_estimated_cost_usd"] = sumCounter(mf)
		}
	}

	json.NewEncoder(w).Encode(stats)
}

func sumCounter(mf *dto.MetricFamily) float64 {
	if mf.GetType() != dto.MetricType_COUNTER {
		return 0
	}
	var total float64
	for _, m := range mf.GetMetric() {
		total += m.GetCounter().GetValue()
	}
	return total
}

func labelValue(m *dto.Metric, name string) string {
	for _, lp := range m.GetLabel() {
		if lp.GetName() == name {
			return lp.GetValue()
		}
	}
	return ""
}

