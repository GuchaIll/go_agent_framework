package core

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewGraphAndAdders(t *testing.T) {
	graph := NewGraph("workflow").
		AddSerial(&fakeAgent{name: "first"}).
		AddParallel(&fakeAgent{name: "second"}, &fakeAgent{name: "third"})

	if graph.name != "workflow" {
		t.Fatalf("expected graph name workflow, got %q", graph.name)
	}
	if len(graph.steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(graph.steps))
	}
	if graph.steps[0].Name != "first" {
		t.Fatalf("expected first step name first, got %q", graph.steps[0].Name)
	}
	if graph.steps[1].Name != "second+third" {
		t.Fatalf("expected parallel step name second+third, got %q", graph.steps[1].Name)
	}
}

func TestGraphRunEmptyGraph(t *testing.T) {
	if err := NewGraph("empty").Run(newTestContext()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestGraphRunSerialOrder(t *testing.T) {
	var mu sync.Mutex
	var calls []string

	record := func(name string) func(*Context) error {
		return func(*Context) error {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, name)
			return nil
		}
	}

	graph := NewGraph("serial").
		AddSerial(&fakeAgent{name: "first", run: record("first")}).
		AddSerial(&fakeAgent{name: "second", run: record("second")}).
		AddSerial(&fakeAgent{name: "third", run: record("third")})

	if err := graph.Run(newTestContext()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	want := []string{"first", "second", "third"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("expected call order %v, got %v", want, calls)
	}
}

func TestGraphRunStopsOnSerialFailure(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	sentinel := errors.New("boom")

	record := func(name string, err error) *fakeAgent {
		return &fakeAgent{
			name: name,
			run: func(*Context) error {
				mu.Lock()
				defer mu.Unlock()
				calls = append(calls, name)
				return err
			},
		}
	}

	graph := NewGraph("serial-fail").
		AddSerial(record("first", nil)).
		AddSerial(record("second", sentinel)).
		AddSerial(record("third", nil))

	err := graph.Run(newTestContext())
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped error %v, got %v", sentinel, err)
	}
	if !strings.Contains(err.Error(), "agent second failed") {
		t.Fatalf("expected serial failure message, got %v", err)
	}

	want := []string{"first", "second"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("expected calls %v, got %v", want, calls)
	}
}

func TestGraphRunParallelExecutesConcurrently(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	errCh := make(chan error, 1)

	makeAgent := func(name string) *fakeAgent {
		return &fakeAgent{
			name: name,
			run: func(*Context) error {
				started <- name
				<-release
				return nil
			},
		}
	}

	graph := NewGraph("parallel").
		AddParallel(makeAgent("left"), makeAgent("right"))

	go func() {
		errCh <- graph.Run(newTestContext())
	}()

	waitForStart := func(label string) {
		t.Helper()
		select {
		case <-started:
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timed out waiting for %s to start", label)
		}
	}

	waitForStart("first agent")
	waitForStart("second agent")
	close(release)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("graph run did not finish after releasing parallel agents")
	}
}

func TestGraphRunParallelReturnsError(t *testing.T) {
	sentinel := errors.New("parallel boom")
	graph := NewGraph("parallel-fail").AddParallel(
		&fakeAgent{name: "ok"},
		&fakeAgent{name: "bad", run: func(*Context) error { return sentinel }},
	)

	err := graph.Run(newTestContext())
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped error %v, got %v", sentinel, err)
	}
	if !strings.Contains(err.Error(), "parallel step ok+bad") {
		t.Fatalf("expected parallel step error message, got %v", err)
	}
}
