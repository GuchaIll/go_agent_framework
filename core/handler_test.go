package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestHandleRequestInvalidJSON(t *testing.T) {
	orchestrator := NewOrchestrator(NewGraph("test"), newFakeStateStore(), 1)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))
	rec := httptest.NewRecorder()

	orchestrator.HandleRequest(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid JSON body") {
		t.Fatalf("expected invalid JSON message, got %q", rec.Body.String())
	}
}

func TestHandleRequestGeneratesSessionIDAndReturnsState(t *testing.T) {
	store := newFakeStateStore()
	graph := NewGraph("test").AddSerial(&fakeAgent{
		name: "enrich",
		run: func(ctx *Context) error {
			ctx.State["result"] = "ok"
			return nil
		},
	})
	orchestrator := NewOrchestrator(graph, store, 1)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"foo":"bar"}`))
	rec := httptest.NewRecorder()

	orchestrator.HandleRequest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	sessionID := rec.Header().Get("X-Session-ID")
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(sessionID) {
		t.Fatalf("expected generated session ID, got %q", sessionID)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("expected json content type, got %q", got)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["foo"] != "bar" || body["result"] != "ok" {
		t.Fatalf("unexpected response body: %v", body)
	}
	if store.putCalls != 1 {
		t.Fatalf("expected one put call, got %d", store.putCalls)
	}
}

func TestHandleRequestUsesProvidedSessionIDAndMergesState(t *testing.T) {
	store := newFakeStateStore()
	store.sessions["sess-1"] = &Session{
		ID:    "sess-1",
		State: map[string]interface{}{"keep": "yes", "overwrite": "old"},
	}
	graph := NewGraph("test").AddSerial(&fakeAgent{
		name: "enrich",
		run: func(ctx *Context) error {
			ctx.State["agent"] = "ran"
			return nil
		},
	})
	orchestrator := NewOrchestrator(graph, store, 1)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"overwrite":"new","fresh":true}`))
	req.Header.Set("X-Session-ID", "sess-1")
	rec := httptest.NewRecorder()

	orchestrator.HandleRequest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("X-Session-ID") != "sess-1" {
		t.Fatalf("expected provided session ID, got %q", rec.Header().Get("X-Session-ID"))
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["keep"] != "yes" || body["overwrite"] != "new" || body["fresh"] != true || body["agent"] != "ran" {
		t.Fatalf("unexpected merged state: %v", body)
	}
}

func TestHandleRequestGraphFailure(t *testing.T) {
	store := newFakeStateStore()
	sentinel := errors.New("boom")
	graph := NewGraph("test").AddSerial(&fakeAgent{
		name: "failer",
		run:  func(*Context) error { return sentinel },
	})
	orchestrator := NewOrchestrator(graph, store, 1)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"foo":"bar"}`))
	rec := httptest.NewRecorder()

	orchestrator.HandleRequest(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "agent failer failed") {
		t.Fatalf("expected graph failure body, got %q", rec.Body.String())
	}
	if store.putCalls != 0 {
		t.Fatalf("expected no save on graph failure, got %d puts", store.putCalls)
	}
}

func TestHandleRequestStoreFailures(t *testing.T) {
	t.Run("load failure", func(t *testing.T) {
		store := newFakeStateStore()
		store.getErr = errors.New("load failed")
		orchestrator := NewOrchestrator(NewGraph("test"), store, 1)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()

		orchestrator.HandleRequest(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "failed to load session") {
			t.Fatalf("expected load failure message, got %q", rec.Body.String())
		}
	})

	t.Run("save failure", func(t *testing.T) {
		store := newFakeStateStore()
		store.putErr = errors.New("save failed")
		graph := NewGraph("test").AddSerial(&fakeAgent{name: "ok"})
		orchestrator := NewOrchestrator(graph, store, 1)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()

		orchestrator.HandleRequest(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "failed to save session") {
			t.Fatalf("expected save failure message, got %q", rec.Body.String())
		}
	})
}

func TestHandleAsyncRequest(t *testing.T) {
	t.Run("service unavailable", func(t *testing.T) {
		orchestrator := NewOrchestrator(NewGraph("test"), newFakeStateStore(), 1)
		req := httptest.NewRequest(http.MethodPost, "/async", strings.NewReader(`{"graph_name":"g"}`))
		rec := httptest.NewRecorder()

		orchestrator.HandleAsyncRequest(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		orchestrator := NewOrchestrator(NewGraph("test"), newFakeStateStore(), 1)
		orchestrator.Enqueuer = &fakeEnqueuer{}
		orchestrator.JobStore = &fakeJobGetter{}
		req := httptest.NewRequest(http.MethodPost, "/async", strings.NewReader("{"))
		rec := httptest.NewRecorder()

		orchestrator.HandleAsyncRequest(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing graph name", func(t *testing.T) {
		orchestrator := NewOrchestrator(NewGraph("test"), newFakeStateStore(), 1)
		orchestrator.Enqueuer = &fakeEnqueuer{}
		orchestrator.JobStore = &fakeJobGetter{}
		req := httptest.NewRequest(http.MethodPost, "/async", strings.NewReader(`{"input":{"q":"x"}}`))
		rec := httptest.NewRecorder()

		orchestrator.HandleAsyncRequest(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "graph_name is required") {
			t.Fatalf("expected missing graph name message, got %q", rec.Body.String())
		}
	})

	t.Run("enqueue failure", func(t *testing.T) {
		orchestrator := NewOrchestrator(NewGraph("test"), newFakeStateStore(), 1)
		orchestrator.Enqueuer = &fakeEnqueuer{err: errors.New("queue down")}
		orchestrator.JobStore = &fakeJobGetter{}
		req := httptest.NewRequest(http.MethodPost, "/async", strings.NewReader(`{"graph_name":"g"}`))
		rec := httptest.NewRecorder()

		orchestrator.HandleAsyncRequest(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "failed to enqueue") {
			t.Fatalf("expected enqueue failure message, got %q", rec.Body.String())
		}
	})

	t.Run("accepted response", func(t *testing.T) {
		enqueuer := &fakeEnqueuer{}
		orchestrator := NewOrchestrator(NewGraph("test"), newFakeStateStore(), 1)
		orchestrator.Enqueuer = enqueuer
		orchestrator.JobStore = &fakeJobGetter{}
		req := httptest.NewRequest(http.MethodPost, "/async", strings.NewReader(`{"graph_name":"docqa","input":{"q":"hello"}}`))
		rec := httptest.NewRecorder()

		orchestrator.HandleAsyncRequest(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", rec.Code)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if body["status"] != "PENDING" || body["job_id"] == "" || body["session_id"] == "" {
			t.Fatalf("unexpected async response body: %v", body)
		}
		if len(enqueuer.jobs) != 1 {
			t.Fatalf("expected one enqueued job, got %d", len(enqueuer.jobs))
		}
		job, ok := enqueuer.jobs[0].(*asyncJobEnvelope)
		if !ok {
			t.Fatalf("expected async job envelope, got %T", enqueuer.jobs[0])
		}
		if job.graphName != "docqa" || job.id == "" || job.sessionID == "" {
			t.Fatalf("unexpected enqueued job: %+v", job)
		}
	})
}

func TestHandleJobStatus(t *testing.T) {
	t.Run("service unavailable", func(t *testing.T) {
		orchestrator := NewOrchestrator(NewGraph("test"), newFakeStateStore(), 1)
		req := httptest.NewRequest(http.MethodGet, "/jobs?job_id=job-1", nil)
		rec := httptest.NewRecorder()

		orchestrator.HandleJobStatus(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("missing job id", func(t *testing.T) {
		orchestrator := NewOrchestrator(NewGraph("test"), newFakeStateStore(), 1)
		orchestrator.JobStore = &fakeJobGetter{}
		req := httptest.NewRequest(http.MethodGet, "/jobs", nil)
		rec := httptest.NewRecorder()

		orchestrator.HandleJobStatus(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		orchestrator := NewOrchestrator(NewGraph("test"), newFakeStateStore(), 1)
		orchestrator.JobStore = &fakeJobGetter{err: errors.New("missing")}
		req := httptest.NewRequest(http.MethodGet, "/jobs?job_id=job-1", nil)
		rec := httptest.NewRecorder()

		orchestrator.HandleJobStatus(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "job not found") {
			t.Fatalf("expected not found message, got %q", rec.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		getter := &fakeJobGetter{
			job: map[string]interface{}{"job_id": "job-1", "status": "COMPLETED"},
		}
		orchestrator := NewOrchestrator(NewGraph("test"), newFakeStateStore(), 1)
		orchestrator.JobStore = getter
		req := httptest.NewRequest(http.MethodGet, "/jobs?job_id=job-1", nil)
		rec := httptest.NewRecorder()

		orchestrator.HandleJobStatus(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if len(getter.ids) != 1 || getter.ids[0] != "job-1" {
			t.Fatalf("expected job lookup for job-1, got %v", getter.ids)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if body["job_id"] != "job-1" || body["status"] != "COMPLETED" {
			t.Fatalf("unexpected job status body: %v", body)
		}
	})
}

func TestPickOutputFields(t *testing.T) {
	got := PickOutputFields(
		map[string]interface{}{"keep": 1, "also": "x"},
		"keep", "missing",
	)

	if len(got) != 1 {
		t.Fatalf("expected one output field, got %v", got)
	}
	if got["keep"] != 1 {
		t.Fatalf("expected keep=1, got %v", got["keep"])
	}
	if _, ok := got["missing"]; ok {
		t.Fatalf("did not expect missing key, got %v", got)
	}
}
