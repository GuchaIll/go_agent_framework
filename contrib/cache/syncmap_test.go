package cache

import (
	"testing"
	"time"
)

func TestSyncMapCacheMissOnEmptyCache(t *testing.T) {
	cache := NewSyncMapCache()

	if got, ok := cache.Get("missing"); ok || got != nil {
		t.Fatalf("expected empty cache miss, got value=%v ok=%v", got, ok)
	}
}

func TestSyncMapCacheSetGetAndDelete(t *testing.T) {
	cache := NewSyncMapCache()
	cache.Set("key", "value", 0)

	got, ok := cache.Get("key")
	if !ok || got != "value" {
		t.Fatalf("expected cached value, got value=%v ok=%v", got, ok)
	}

	cache.Delete("key")
	if got, ok := cache.Get("key"); ok || got != nil {
		t.Fatalf("expected deleted key miss, got value=%v ok=%v", got, ok)
	}
}

func TestSyncMapCacheTTLExpiry(t *testing.T) {
	cache := NewSyncMapCache()
	cache.Set("ttl", "value", 20*time.Millisecond)

	time.Sleep(30 * time.Millisecond)

	if got, ok := cache.Get("ttl"); ok || got != nil {
		t.Fatalf("expected expired key miss, got value=%v ok=%v", got, ok)
	}
}

func TestSyncMapCacheZeroTTLDoesNotExpire(t *testing.T) {
	cache := NewSyncMapCache()
	cache.Set("steady", "value", 0)

	time.Sleep(20 * time.Millisecond)

	got, ok := cache.Get("steady")
	if !ok || got != "value" {
		t.Fatalf("expected non-expiring value, got value=%v ok=%v", got, ok)
	}
}
