package cache

import (
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type entry struct {
	value     []byte
	expiresAt time.Time
}

// Cache is a thread-safe in-memory store with per-entry TTL and LRU eviction.
type Cache struct {
	lru    *lru.Cache[string, entry]
	hits   atomic.Int64
	misses atomic.Int64
}

// Stats holds diagnostic counters for a Cache.
type Stats struct {
	Hits   int64 `json:"hits"`
	Misses int64 `json:"misses"`
	Size   int   `json:"size"`
}

// New creates a Cache that holds at most maxSize live entries.
func New(maxSize int) (*Cache, error) {
	l, err := lru.New[string, entry](maxSize)
	if err != nil {
		return nil, err
	}
	return &Cache{lru: l}, nil
}

// Get returns the cached bytes for key. Returns false if the entry is absent or expired.
func (c *Cache) Get(key string) ([]byte, bool) {
	e, ok := c.lru.Get(key)
	if !ok || time.Now().UTC().After(e.expiresAt) {
		if ok {
			c.lru.Remove(key)
		}
		c.misses.Add(1)
		return nil, false
	}
	c.hits.Add(1)
	return e.value, true
}

// Set stores value under key, expiring after ttl.
func (c *Cache) Set(key string, value []byte, ttl time.Duration) {
	c.lru.Add(key, entry{value: value, expiresAt: time.Now().UTC().Add(ttl)})
}

// Stats returns a snapshot of hit/miss counters and the current entry count.
func (c *Cache) Stats() Stats {
	return Stats{
		Hits:   c.hits.Load(),
		Misses: c.misses.Load(),
		Size:   c.lru.Len(),
	}
}
