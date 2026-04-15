package core

import "testing"

func TestMemStoreGetMissingSession(t *testing.T) {
	store := NewMemStore()

	session, err := store.Get("missing")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if session.ID != "missing" {
		t.Fatalf("expected session ID missing, got %q", session.ID)
	}
	if session.Version != 0 {
		t.Fatalf("expected version 0, got %d", session.Version)
	}
	if session.State == nil {
		t.Fatal("expected initialized state map")
	}
	if len(session.State) != 0 {
		t.Fatalf("expected empty state, got %v", session.State)
	}
}

func TestMemStorePutAndGetRoundTrip(t *testing.T) {
	store := NewMemStore()
	session := &Session{
		ID:    "sess-1",
		State: map[string]interface{}{"answer": 42},
	}

	if err := store.Put(session); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if session.Version != 1 {
		t.Fatalf("expected version 1 after first put, got %d", session.Version)
	}
	if session.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set")
	}

	got, err := store.Get("sess-1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.State["answer"] != 42 {
		t.Fatalf("expected persisted state, got %v", got.State)
	}
	if got.Version != 1 {
		t.Fatalf("expected version 1, got %d", got.Version)
	}
}

func TestMemStorePutOverwriteIncrementsVersion(t *testing.T) {
	store := NewMemStore()
	session := &Session{
		ID:    "sess-2",
		State: map[string]interface{}{"value": "first"},
	}

	if err := store.Put(session); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	firstUpdatedAt := session.UpdatedAt

	session.State["value"] = "second"
	if err := store.Put(session); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if session.Version != 2 {
		t.Fatalf("expected version 2 after overwrite, got %d", session.Version)
	}
	if session.UpdatedAt.Before(firstUpdatedAt) {
		t.Fatalf("expected UpdatedAt to advance or stay equal, got %v then %v", firstUpdatedAt, session.UpdatedAt)
	}

	got, err := store.Get("sess-2")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if got.State["value"] != "second" {
		t.Fatalf("expected overwritten value, got %v", got.State["value"])
	}
}
