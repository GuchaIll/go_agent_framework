package core

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestWorkerRunExitsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	worker := &Worker{
		Logger: discardLogger(),
	}

	if err := worker.Run(ctx); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestWorkerProcessOneDequeueError(t *testing.T) {
	sentinel := errors.New("dequeue failed")
	worker := &Worker{
		Queue:  &fakeWorkerQueue{dequeueErr: sentinel},
		Logger: discardLogger(),
	}

	err := worker.processOne(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected dequeue error %v, got %v", sentinel, err)
	}
}

func TestWorkerProcessOneNilJobNoop(t *testing.T) {
	queue := &fakeWorkerQueue{}
	worker := &Worker{
		Queue:  queue,
		Logger: discardLogger(),
	}

	if err := worker.processOne(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if queue.ackCalls != 0 {
		t.Fatalf("expected no ack for nil job, got %d", queue.ackCalls)
	}
}

func TestWorkerProcessOneUnknownGraph(t *testing.T) {
	queue := &fakeWorkerQueue{
		job: &fakeWorkerJob{id: "job-1", graphName: "missing", sessionID: "sess-1"},
	}
	jobs := &fakeJobUpdater{}
	worker := &Worker{
		Graphs: map[string]*Graph{},
		Queue:  queue,
		Jobs:   jobs,
		Logger: discardLogger(),
	}

	if err := worker.processOne(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if queue.ackCalls != 1 {
		t.Fatalf("expected one ack, got %d", queue.ackCalls)
	}
	if len(jobs.statuses) != 0 {
		t.Fatalf("expected no running status for unknown graph, got %v", jobs.statuses)
	}
	if len(jobs.results) != 1 || !strings.Contains(jobs.results[0].jobErr, "unknown graph: missing") {
		t.Fatalf("expected unknown graph result, got %+v", jobs.results)
	}
	if jobs.results[0].result != nil {
		t.Fatalf("expected nil result state, got %v", jobs.results[0].result)
	}
}

func TestWorkerProcessOneLoadSessionFailure(t *testing.T) {
	queue := &fakeWorkerQueue{
		job: &fakeWorkerJob{id: "job-1", graphName: "known", sessionID: "sess-1"},
	}
	store := newFakeStateStore()
	store.getErr = errors.New("store down")
	jobs := &fakeJobUpdater{}
	worker := &Worker{
		Graphs: map[string]*Graph{"known": NewGraph("known")},
		Store:  store,
		Queue:  queue,
		Jobs:   jobs,
		Logger: discardLogger(),
	}

	if err := worker.processOne(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if queue.ackCalls != 1 {
		t.Fatalf("expected one ack, got %d", queue.ackCalls)
	}
	if len(jobs.statuses) != 1 || jobs.statuses[0].status != "RUNNING" {
		t.Fatalf("expected running status update, got %+v", jobs.statuses)
	}
	if len(jobs.results) != 1 || !strings.Contains(jobs.results[0].jobErr, "load session: store down") {
		t.Fatalf("expected load session failure result, got %+v", jobs.results)
	}
}

func TestWorkerProcessOneGraphFailure(t *testing.T) {
	queue := &fakeWorkerQueue{
		job: &fakeWorkerJob{
			id:        "job-1",
			graphName: "known",
			sessionID: "sess-1",
			input:     map[string]interface{}{"question": "hi"},
		},
	}
	store := newFakeStateStore()
	store.sessions["sess-1"] = &Session{ID: "sess-1", State: map[string]interface{}{"existing": true}}
	sentinel := errors.New("graph failed")
	graph := NewGraph("known").AddSerial(&fakeAgent{
		name: "failer",
		run: func(ctx *Context) error {
			ctx.State["seen"] = "yes"
			return sentinel
		},
	})
	jobs := &fakeJobUpdater{}
	worker := &Worker{
		Graphs: map[string]*Graph{"known": graph},
		Store:  store,
		Queue:  queue,
		Jobs:   jobs,
		Logger: discardLogger(),
	}

	if err := worker.processOne(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if queue.ackCalls != 1 {
		t.Fatalf("expected one ack, got %d", queue.ackCalls)
	}
	if store.putCalls != 0 {
		t.Fatalf("expected no save on graph failure, got %d puts", store.putCalls)
	}
	if len(jobs.results) != 1 {
		t.Fatalf("expected one result update, got %+v", jobs.results)
	}
	if !strings.Contains(jobs.results[0].jobErr, "graph failed") {
		t.Fatalf("expected graph failure result, got %+v", jobs.results[0])
	}
	if jobs.results[0].result["question"] != "hi" || jobs.results[0].result["existing"] != true || jobs.results[0].result["seen"] != "yes" {
		t.Fatalf("expected merged failure state, got %v", jobs.results[0].result)
	}
}

func TestWorkerProcessOneSuccessMergesInputAndTracksStatusOrder(t *testing.T) {
	queue := &fakeWorkerQueue{
		job: &fakeWorkerJob{
			id:        "job-1",
			graphName: "known",
			sessionID: "sess-1",
			input:     map[string]interface{}{"override": "new", "input": "present"},
		},
	}
	store := newFakeStateStore()
	store.sessions["sess-1"] = &Session{
		ID:    "sess-1",
		State: map[string]interface{}{"existing": "yes", "override": "old"},
	}
	order := []string{}
	jobs := &fakeJobUpdater{order: &order}
	graph := NewGraph("known").AddSerial(&fakeAgent{
		name: "assert",
		run: func(ctx *Context) error {
			order = append(order, "agent")
			if ctx.State["existing"] != "yes" {
				return errors.New("missing existing state")
			}
			if ctx.State["override"] != "new" || ctx.State["input"] != "present" {
				return errors.New("input was not merged before graph execution")
			}
			ctx.State["output"] = "done"
			return nil
		},
	})
	worker := &Worker{
		Graphs: map[string]*Graph{"known": graph},
		Store:  store,
		Queue:  queue,
		Jobs:   jobs,
		Logger: discardLogger(),
	}

	if err := worker.processOne(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if queue.ackCalls != 1 {
		t.Fatalf("expected one ack, got %d", queue.ackCalls)
	}
	if store.putCalls != 1 {
		t.Fatalf("expected one save, got %d", store.putCalls)
	}
	if len(jobs.statuses) != 1 || jobs.statuses[0].status != "RUNNING" {
		t.Fatalf("expected RUNNING status, got %+v", jobs.statuses)
	}
	if len(jobs.results) != 1 || jobs.results[0].jobErr != "" {
		t.Fatalf("expected successful result update, got %+v", jobs.results)
	}
	if jobs.results[0].result["output"] != "done" {
		t.Fatalf("expected output in result state, got %v", jobs.results[0].result)
	}
	wantOrder := []string{"status:RUNNING", "agent", "result"}
	if !reflect.DeepEqual(order, wantOrder) {
		t.Fatalf("expected order %v, got %v", wantOrder, order)
	}
}

func TestWorkerProcessOneSaveFailureStillSetsResult(t *testing.T) {
	queue := &fakeWorkerQueue{
		job: &fakeWorkerJob{id: "job-1", graphName: "known", sessionID: "sess-1"},
	}
	store := newFakeStateStore()
	store.sessions["sess-1"] = &Session{ID: "sess-1", State: map[string]interface{}{}}
	store.putErr = errors.New("save failed")
	jobs := &fakeJobUpdater{}
	graph := NewGraph("known").AddSerial(&fakeAgent{
		name: "ok",
		run: func(ctx *Context) error {
			ctx.State["saved"] = false
			return nil
		},
	})
	worker := &Worker{
		Graphs: map[string]*Graph{"known": graph},
		Store:  store,
		Queue:  queue,
		Jobs:   jobs,
		Logger: discardLogger(),
	}

	if err := worker.processOne(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if store.putCalls != 1 {
		t.Fatalf("expected one put attempt, got %d", store.putCalls)
	}
	if len(jobs.results) != 1 || jobs.results[0].jobErr != "" {
		t.Fatalf("expected successful result despite save failure, got %+v", jobs.results)
	}
}

func TestWorkerProcessOneAckFailureIsLoggedButNotReturned(t *testing.T) {
	queue := &fakeWorkerQueue{
		job:    &fakeWorkerJob{id: "job-1", graphName: "known", sessionID: "sess-1"},
		ackErr: errors.New("ack failed"),
	}
	store := newFakeStateStore()
	store.sessions["sess-1"] = &Session{ID: "sess-1", State: map[string]interface{}{}}
	worker := &Worker{
		Graphs: map[string]*Graph{"known": NewGraph("known")},
		Store:  store,
		Queue:  queue,
		Logger: discardLogger(),
	}

	if err := worker.processOne(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if queue.ackCalls != 1 {
		t.Fatalf("expected one ack attempt, got %d", queue.ackCalls)
	}
}
