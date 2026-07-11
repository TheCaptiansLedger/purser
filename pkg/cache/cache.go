// Package cache defines the Cache port shared by any part of Purser that
// needs to cache arbitrary byte values (e.g. third-party provider
// responses), independent of storage backend. See pkg/cache/memory for the
// in-memory adapter; a Redis (or similar) adapter can be added later behind
// the same port without changing any caller.
package cache

import (
	"context"
	"errors"
	"time"
)

// Cache is the port callers depend on. Implementations must be safe for
// concurrent use by multiple goroutines.
type Cache interface {
	// Get returns the cached value for key. The second return value is
	// false if key is absent or has expired.
	Get(ctx context.Context, key string) (value []byte, ok bool, err error)

	// Set stores value under key, replacing any existing entry. The entry
	// expires after the cache's configured DefaultTTL (zero means it never
	// expires on its own, though it may still be evicted under
	// MaxItems/MaxBytes pressure).
	Set(ctx context.Context, key string, value []byte) error

	// Delete removes key, if present. Deleting an absent key is not an
	// error.
	Delete(ctx context.Context, key string) error

	// Len reports the current number of entries.
	Len(ctx context.Context) (int, error)

	// Close releases any resources held by the cache. After Close, all
	// other methods return ErrClosed.
	Close() error
}

// ErrClosed is returned by Cache methods once Close has been called.
var ErrClosed = errors.New("cache: closed")

// Config configures a single named Cache instance. Every field has a sane
// default via DefaultConfig — callers only need to override what differs
// for their instance. Field names/tags follow the PURSER_<NESTED>_<KEY>
// convention documented in ADR 0010 for future Viper embedding.
type Config struct {
	// MaxItems is the maximum number of entries the cache holds before it
	// evicts the least recently used entry. Must be > 0.
	MaxItems int `mapstructure:"max_items"`

	// MaxBytes is the approximate maximum total size, in bytes, of all
	// cached keys and values before least-recently-used entries are
	// evicted to make room. Zero means unlimited.
	MaxBytes int64 `mapstructure:"max_bytes"`

	// DefaultTTL is how long an entry stays valid after being set. Zero
	// means entries never expire on their own.
	DefaultTTL time.Duration `mapstructure:"default_ttl"`
}

// DefaultConfig returns the sane defaults every Cache instance starts from.
func DefaultConfig() Config {
	return Config{
		MaxItems:   10_000,
		MaxBytes:   64 * 1024 * 1024, // 64 MiB
		DefaultTTL: 15 * time.Minute,
	}
}
