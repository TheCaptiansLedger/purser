// Package cacheregistry is the thin adapter implementing
// ports.CacheRegistry by wrapping a fixed set of named pkg/cache.Cache
// instances — the hexagonal seam those already-constructed caches are
// exposed to the API layer through, per
// docs/adr/0001-hexagonal-architecture.md. One concrete type, built once at
// composition-root time; see internal/adapters/jobqueue for the same
// single-implementation, direct-test-file shape.
package cacheregistry

import (
	"purser/internal/ports"
	"purser/pkg/cache"
	"sort"
)

// Registry implements ports.CacheRegistry over a fixed map of named caches.
type Registry struct {
	caches map[string]cache.Cache
	names  []string
}

var _ ports.CacheRegistry = (*Registry)(nil)

// New builds a Registry from caches, keyed by the name each was constructed
// with (e.g. "stashdb", "musicbrainz"). The set of names is fixed at
// construction — Registry has no way to add or remove an entry afterward.
func New(caches map[string]cache.Cache) *Registry {
	names := make([]string, 0, len(caches))
	for name := range caches {
		names = append(names, name)
	}
	sort.Strings(names)

	return &Registry{caches: caches, names: names}
}

// Names implements ports.CacheRegistry.
func (r *Registry) Names() []string {
	names := make([]string, len(r.names))
	copy(names, r.names)
	return names
}

// Cache implements ports.CacheRegistry.
func (r *Registry) Cache(name string) (cache.Cache, bool) {
	c, ok := r.caches[name]
	return c, ok
}
