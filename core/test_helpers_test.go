package core

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
)

type fakeAgent struct {
	name string
	run  func(*Context) error
}

func (f *fakeAgent) Name() string { return f.name }

func (f *fakeAgent) Run(ctx *Context) error {
	if f.run != nil {
		return f.run(ctx)
	}
	return nil
}

type fakeStateStore struct {
	mu       sync.Mutex
	sessions map[string]*Session
	getErr   error
	putErr   error
	getCalls []string
	putCalls int
	lastPut  *Session
}

func newFakeStateStore() *fakeStateStore {
	return &fakeStateStore{sessions: make(map[string]*Session)}
}

func (f *fakeStateStore) Get(id string) (*Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.getCalls = append(f.getCalls, id)
	if f.getErr != nil {
		return nil, f.getErr
	}
	if session, ok := f.sessions[id]; ok {
		return session, nil
	}
	session := &Session{ID: id, State: map[string]interface{}{}}
	f.sessions[id] = session
	return session, nil
}

func (f *fakeStateStore) Put(session *Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.putCalls++
	f.lastPut = session
	if f.putErr != nil {
		return f.putErr
	}
	f.sessions[session.ID] = session
	return nil
}

type fakeEnqueuer struct {
	err  error
	jobs []AsyncJob
}

func (f *fakeEnqueuer) Enqueue(_ context.Context, job AsyncJob) error {
	if f.err != nil {
		return f.err
	}
	f.jobs = append(f.jobs, job)
	return nil
}

type fakeJobGetter struct {
	job interface{}
	err error
	ids []string
}

func (f *fakeJobGetter) GetJob(_ context.Context, jobID string) (interface{}, error) {
	f.ids = append(f.ids, jobID)
	if f.err != nil {
		return nil, f.err
	}
	return f.job, nil
}

type fakeWorkerJob struct {
	id        string
	graphName string
	sessionID string
	input     map[string]interface{}
}

func (f *fakeWorkerJob) GetID() string                    { return f.id }
func (f *fakeWorkerJob) GetGraphName() string             { return f.graphName }
func (f *fakeWorkerJob) GetSessionID() string             { return f.sessionID }
func (f *fakeWorkerJob) GetInput() map[string]interface{} { return f.input }

type fakeWorkerQueue struct {
	job          WorkerJob
	dequeueErr   error
	ackErr       error
	dequeueCalls int
	ackCalls     int
	ackedJobs    []WorkerJob
}

func (f *fakeWorkerQueue) Dequeue(context.Context) (WorkerJob, error) {
	f.dequeueCalls++
	if f.dequeueErr != nil {
		return nil, f.dequeueErr
	}
	return f.job, nil
}

func (f *fakeWorkerQueue) Ack(_ context.Context, job WorkerJob) error {
	f.ackCalls++
	f.ackedJobs = append(f.ackedJobs, job)
	return f.ackErr
}

type statusCall struct {
	jobID  string
	status string
}

type resultCall struct {
	jobID  string
	result map[string]interface{}
	jobErr string
}

type fakeJobUpdater struct {
	statuses []statusCall
	results  []resultCall
	order    *[]string
}

func (f *fakeJobUpdater) UpdateStatus(_ context.Context, jobID string, status string) error {
	f.statuses = append(f.statuses, statusCall{jobID: jobID, status: status})
	if f.order != nil {
		*f.order = append(*f.order, "status:"+status)
	}
	return nil
}

func (f *fakeJobUpdater) SetResult(_ context.Context, jobID string, result map[string]interface{}, jobErr string) error {
	f.results = append(f.results, resultCall{jobID: jobID, result: cloneState(result), jobErr: jobErr})
	if f.order != nil {
		*f.order = append(*f.order, "result")
	}
	return nil
}

func newTestContext() *Context {
	return &Context{
		SessionID: "test-session",
		State:     map[string]interface{}{},
		Logger:    discardLogger(),
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
}

func bufferedLogger() (*slog.Logger, *bytes.Buffer) {
	buf := bytes.NewBuffer(nil)
	return slog.New(slog.NewTextHandler(buf, nil)), buf
}

func cloneState(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
