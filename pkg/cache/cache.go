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
// Byte accounting is maintained via the LRU eviction callback so all eviction
// paths (TTL expiry, LRU pressure, explicit Flush) are covered automatically.
type Cache struct {
	name   string
	lru    *lru.Cache[string, entry]
	hits   atomic.Int64
	misses atomic.Int64
	bytes  atomic.Int64
}

// Stats holds diagnostic counters for a single named Cache.
type Stats struct {
	Name   string `json:"name"`
	Hits   int64  `json:"hits"`
	Misses int64  `json:"misses"`
	Size   int    `json:"size"`
	Bytes  int64  `json:"bytes"`
}

// New creates a named Cache that holds at most maxSize live entries.
func New(name string, maxSize int) (*Cache, error) {
	c := &Cache{name: name}
	l, err := lru.NewWithEvict[string, entry](maxSize, func(_ string, e entry) {
		c.bytes.Add(-int64(len(e.value)))
	})
	if err != nil {
		return nil, err
	}
	c.lru = l
	return c, nil
}

// Name returns the cache's identifier.
func (c *Cache) Name() string { return c.name }

// Get returns the cached bytes for key. Returns false if the entry is absent or expired.
func (c *Cache) Get(key string) ([]byte, bool) {
	e, ok := c.lru.Get(key)
	if !ok || time.Now().UTC().After(e.expiresAt) {
		if ok {
			c.lru.Remove(key) // triggers eviction callback → decrements bytes
		}
		c.misses.Add(1)
		return nil, false
	}
	c.hits.Add(1)
	return e.value, true
}

// Set stores value under key, expiring after ttl.
// The hashicorp thread-safe wrapper only fires onEvictedCB when Add returns
// evicted=true (capacity pressure), not for overwrites. We correct the byte
// counter for overwrites by peeking at the displaced entry before replacing it.
func (c *Cache) Set(key string, value []byte, ttl time.Duration) {
	if old, ok := c.lru.Peek(key); ok {
		c.bytes.Add(-int64(len(old.value)))
	}
	c.bytes.Add(int64(len(value)))
	c.lru.Add(key, entry{value: value, expiresAt: time.Now().UTC().Add(ttl)})
}

// Flush removes all entries. The LRU eviction callback fires for every entry,
// bringing the byte counter back to zero.
func (c *Cache) Flush() {
	c.lru.Purge()
}

// Stats returns a snapshot of the cache name, hit/miss counters, byte usage,
// and current live-entry count.
func (c *Cache) Stats() Stats {
	return Stats{
		Name:   c.name,
		Hits:   c.hits.Load(),
		Misses: c.misses.Load(),
		Size:   c.lru.Len(),
		Bytes:  c.bytes.Load(),
	}
}
