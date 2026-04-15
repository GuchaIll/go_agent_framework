# LLM Traffic Visualization

This repo’s orchestration framework is centered on `core.Graph` and `core.Orchestrator`.
Graphs define serial and parallel agent steps, while the orchestrator handles HTTP entrypoints, session loading, graph execution, and state persistence.

The Kong guide for visualizing AI traffic with Prometheus and Grafana is gateway-oriented:
https://developer.konghq.com/how-to/visualize-llm-metrics-with-grafana/

This implementation follows the same three-stage pattern, adapted to this project:

1. Capture LLM traffic and execution metadata at the framework boundary.
2. Expose Prometheus metrics from the running example apps.
3. Pre-provision a Grafana dashboard for those metrics.

## What was instrumented

- `core.Graph.Run` now emits graph and agent execution metrics.
- `core.Context.ToContext()` now carries graph, agent, and session metadata into downstream calls.
- `observability.InstrumentLLM(...)` wraps any `contrib/llm.LLMClient` and emits:
  - `go_agent_framework_llm_requests_total`
  - `go_agent_framework_llm_request_duration_seconds`
  - `go_agent_framework_llm_in_flight_requests`
  - `go_agent_framework_llm_prompt_characters_total`
  - `go_agent_framework_llm_completion_characters_total`
  - `go_agent_framework_llm_estimated_tokens_total`
  - `go_agent_framework_llm_estimated_cost_usd_total`

Token and cost metrics are estimates derived from character counts and the per-1K-token prices configured when wrapping the LLM client.

## Run it

Start one or more example servers:

```bash
go run ./examples/chess_coach/cmd
go run ./examples/docqa/cmd
go run ./examples/multi_agent_tools/cmd
```

Each server now exposes Prometheus metrics on `/metrics`:

- `http://localhost:8080/metrics` for chess coach
- `http://localhost:8081/metrics` for docqa
- `http://localhost:8082/metrics` for multi-agent tools

Start Prometheus and Grafana:

```bash
cd deploy/observability
docker compose up
```

Then open:

- Grafana: `http://localhost:3000`
- Prometheus: `http://localhost:9090`

Grafana credentials are:

- Username: `admin`
- Password: `admin`

The dashboard is auto-provisioned as `Go Agent Framework LLM Traffic`.

## Notes

- On macOS and Docker Desktop, Prometheus can scrape the host apps through `host.docker.internal`, which is what `deploy/observability/prometheus.yml` uses.
- If you run these binaries outside the default ports, update the targets in `deploy/observability/prometheus.yml`.
- If you plug in a real LLM backend, keep wrapping it with `observability.InstrumentLLM(...)` so the dashboard continues to work.

## Agent Dashboard

Each example app serves a real-time React dashboard showing the agent DAG,
execution status, and token statistics.

### Production (built-in)

The dashboard is pre-built and served by the Go server automatically:

```bash
# Build the React frontend (one time, or after changes)
cd dashboard && npm install && npm run build && cd ..

# Start an example app
go run ./examples/chess_coach/cmd
```

Then open:

| App               | Dashboard URL                          |
|-------------------|----------------------------------------|
| Chess Coach       | http://localhost:8080/dashboard/       |
| Doc Q&A           | http://localhost:8081/dashboard/       |
| Multi-Agent Tools | http://localhost:8082/dashboard/       |

### Development mode

For live-reloading while working on the dashboard UI:

```bash
cd dashboard
npm run dev
```

This starts Vite on `http://localhost:3001/dashboard/` with API requests proxied
to `http://localhost:8080` (configurable in `dashboard/vite.config.js`).

### Dashboard features

- **Header**: Graph name, LLM call count, prompt/completion/total tokens, estimated cost, connection status
- **DAG visualization**: Agent nodes laid out with dagre, auto-fitted on load
- **Edge styles**: Solid gray for sequential steps, dashed blue (animated) for parallel steps
- **Node states**: Dark gray (idle), white (running, with pulsing green dot), dark red (error)
- **Agent metadata**: Description, capability badges (tools=blue, skills=purple, RAG=green, agents=orange)
- **Sub-process pills**: Appear during tool/skill/RAG execution
- **SSE streaming**: Real-time updates via Server-Sent Events at `/dashboard/events`
