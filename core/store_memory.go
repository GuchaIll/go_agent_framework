package core

import (
	"sync"
	"time"
)

// MemStore is an in-memory StateStore for prototyping.
type MemStore struct {
	mu   sync.RWMutex
	data map[string]*Session
}

// NewMemStore creates an initialized in-memory store.
func NewMemStore() *MemStore {
	return &MemStore{data: make(map[string]*Session)}
}

func (m *MemStore) Get(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.data[id]
	if !ok {
		return &Session{ID: id, State: make(map[string]interface{}), Version: 0}, nil
	}
	return s, nil
}

func (m *MemStore) Put(session *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session.Version++
	session.UpdatedAt = time.Now()
	m.data[session.ID] = session
	return nil
}
