package ports

import "purser/pkg/cache"

// CacheRegistry lets the API layer enumerate and reach every named
// pkg/cache.Cache instance the composition root (cmd/purser) constructed —
// one per external provider adapter (StashDB, MusicBrainz, etc.) — without
// importing any of those adapter packages directly. Modeled on
// JobPublisher/JobReader (internal/ports/job.go): a thin seam over an
// already-abstracted pkg type, per docs/adr/0023-job-queue.md's precedent.
//
// A CacheRegistry implementation is built once, from the caches known at
// startup; it does not support registering or removing a cache afterward —
// see docs/adr/0001-hexagonal-architecture.md and
// docs/adr/0002-solid-design-principles.md.
type CacheRegistry interface {
	// Names returns the name of every registered cache, sorted
	// alphabetically for a stable, deterministic listing order.
	Names() []string

	// Cache returns the named cache.Cache instance. ok is false if no
	// cache was registered under that name.
	Cache(name string) (c cache.Cache, ok bool)
}
