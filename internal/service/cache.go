package service

import (
	"context"
	"fmt"
	"purser/internal/ports"
	"purser/pkg/cache"
)

// CacheStats pairs one registered cache's name with its live pkg/cache.Stats
// snapshot — CacheService's own read shape. Embeds cache.Stats directly
// rather than redeclaring its fields, the same "depend on the pkg type
// directly" precedent JobService follows for pkg/jobqueue.Job/Event, unlike
// internal/config which never leaks past the composition root (see
// internal/service/settings.go's KeyStatus doc comment for that
// distinction).
type CacheStats struct {
	Name string
	cache.Stats
}

// CacheService orchestrates ports.CacheRegistry — reporting stats for and
// flushing the named pkg/cache.Cache instances the composition root
// registered one per external provider adapter. It touches no other
// entity's port, per docs/adr/0011-api-design.md's SRP rule; not a
// shared-kernel entity itself, so it follows DatabaseService/
// SettingsService's admin-RPC shape rather than the Create/Get/Update/
// Delete/List convention those follow.
type CacheService struct {
	registry ports.CacheRegistry
}

// NewCacheService constructs a CacheService backed by registry.
func NewCacheService(registry ports.CacheRegistry) *CacheService {
	return &CacheService{registry: registry}
}

// ListCacheStats returns a live Stats snapshot for every registered cache,
// in registry.Names()'s stable alphabetical order.
func (s *CacheService) ListCacheStats(ctx context.Context) ([]CacheStats, error) {
	names := s.registry.Names()
	out := make([]CacheStats, 0, len(names))
	for _, name := range names {
		// ok is always true here: name came from this same registry's
		// Names() and ports.CacheRegistry's set is fixed at construction
		// (see its own doc comment), so there's no window for it to
		// disappear between the two calls.
		c, _ := s.registry.Cache(name)
		st, err := c.Stats(ctx)
		if err != nil {
			return nil, fmt.Errorf("service: getting stats for cache %q: %w", name, err)
		}
		out = append(out, CacheStats{Name: name, Stats: st})
	}
	return out, nil
}

// FlushCache clears every entry in the named cache, leaving its cumulative
// counters untouched (pkg/cache.Cache.Flush's own contract), and returns
// the name of every cache actually flushed. An empty name flushes every
// registered cache, in registry.Names()'s order, and returns all of them.
// A non-empty, unregistered name returns ports.ErrNotFound.
func (s *CacheService) FlushCache(ctx context.Context, name string) ([]string, error) {
	if name == "" {
		names := s.registry.Names()
		for _, n := range names {
			if err := s.flush(ctx, n); err != nil {
				return nil, err
			}
		}
		return names, nil
	}

	if err := s.flush(ctx, name); err != nil {
		return nil, err
	}
	return []string{name}, nil
}

// flush looks up name in the registry and flushes it, or returns
// ports.ErrNotFound if name isn't registered.
func (s *CacheService) flush(ctx context.Context, name string) error {
	c, ok := s.registry.Cache(name)
	if !ok {
		return fmt.Errorf("%w: unknown cache %q", ports.ErrNotFound, name)
	}
	if err := c.Flush(ctx); err != nil {
		return fmt.Errorf("service: flushing cache %q: %w", name, err)
	}
	return nil
}
