package cache

import (
	"sync"
	"time"
)

// SyncMapCache implements Cache using sync.Map with optional TTL.
type SyncMapCache struct {
	data sync.Map
}

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

func NewSyncMapCache() *SyncMapCache {
	return &SyncMapCache{}
}

func (c *SyncMapCache) Get(key string) (interface{}, bool) {
	raw, ok := c.data.Load(key)
	if !ok {
		return nil, false
	}
	entry := raw.(*cacheEntry)
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.data.Delete(key)
		return nil, false
	}
	return entry.value, true
}

func (c *SyncMapCache) Set(key string, value interface{}, ttl time.Duration) {
	entry := &cacheEntry{value: value}
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	c.data.Store(key, entry)
}

func (c *SyncMapCache) Delete(key string) {
	c.data.Delete(key)
}
