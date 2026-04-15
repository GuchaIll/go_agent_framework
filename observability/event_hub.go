package observability

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Event types published by the framework.
const (
	EventGraphStart      = "graph_start"
	EventGraphEnd        = "graph_end"
	EventAgentStart      = "agent_start"
	EventAgentEnd        = "agent_end"
	EventSubprocessStart = "subprocess_start"
	EventSubprocessEnd   = "subprocess_end"
	EventThought         = "thought"
	EventToolCall        = "tool_call"
	EventToolResult      = "tool_result"
	EventSkillUse        = "skill_use"
	EventDelegation      = "delegation"
	EventChatResponse    = "chat_response"
)

// DashboardEvent is a single SSE payload.
type DashboardEvent struct {
	Type       string                 `json:"type"`
	Timestamp  int64                  `json:"ts"`
	Graph      string                 `json:"graph,omitempty"`
	Agent      string                 `json:"agent,omitempty"`
	Session    string                 `json:"session,omitempty"`
	Status     string                 `json:"status,omitempty"`
	SubProcess string                 `json:"sub_process,omitempty"`
	SubKind    string                 `json:"sub_kind,omitempty"`
	DurationMs float64                `json:"duration_ms,omitempty"`
	Message    string                 `json:"message,omitempty"`
	Detail     map[string]interface{} `json:"detail,omitempty"`
}

// EventHub broadcasts DashboardEvents to connected SSE clients.
type EventHub struct {
	mu          sync.RWMutex
	subscribers map[chan DashboardEvent]struct{}
}

var (
	defaultHub     *EventHub
	defaultHubOnce sync.Once
)

// DefaultHub returns the process-wide event hub singleton.
func DefaultHub() *EventHub {
	defaultHubOnce.Do(func() {
		defaultHub = NewEventHub()
	})
	return defaultHub
}

// NewEventHub creates a new EventHub.
func NewEventHub() *EventHub {
	return &EventHub{
		subscribers: make(map[chan DashboardEvent]struct{}),
	}
}

// Subscribe returns a channel that receives events.
func (h *EventHub) Subscribe() chan DashboardEvent {
	ch := make(chan DashboardEvent, 64)
	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (h *EventHub) Unsubscribe(ch chan DashboardEvent) {
	h.mu.Lock()
	delete(h.subscribers, ch)
	h.mu.Unlock()
	close(ch)
}

// Publish sends an event to all subscribers (non-blocking per subscriber).
func (h *EventHub) Publish(evt DashboardEvent) {
	if evt.Timestamp == 0 {
		evt.Timestamp = time.Now().UnixMilli()
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers {
		select {
		case ch <- evt:
		default:
			// slow consumer — drop
		}
	}
}

// SSEHandler returns an http.HandlerFunc that streams events as SSE.
func (h *EventHub) SSEHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		flusher.Flush()

		ch := h.Subscribe()
		defer h.Unsubscribe(ch)

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-ch:
				if !ok {
					return
				}
				data, _ := json.Marshal(evt)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	}
}
