package ports

import (
	"context"
	"time"
)

// IndexerSearcher is the capability-shaped port for searching whichever
// indexer-aggregator backend is configured (Prowlarr today; a future
// Jackett or NZBHydra2 adapter could satisfy the identical interface). It
// is an ordinary port describing capability, not a
// docs/adr/0027-provider-independence.md-style provider exception: an
// operator configures exactly one active indexer backend, swappable,
// never two competing backends answering the same query. See
// docs/technical/acquisition-indexer-search.md and
// docs/technical/acquisition-pipeline.md.
//
// A zero-result search is a valid empty slice, never ErrNotFound, matching
// every other Search/List method in this codebase (MusicBrainzClient,
// TheAudioDBClient).
type IndexerSearcher interface {
	// Search free-text searches across every indexer the backend has
	// enabled, returned in the backend's own order — no server-side
	// ranking or filtering.
	Search(ctx context.Context, params IndexerSearchParams) ([]IndexerRelease, error)
}

// IndexerSearchParams is the input to IndexerSearcher.Search.
type IndexerSearchParams struct {
	Query      string
	Categories []int // opaque backend category IDs; interpreting them is the adapter's job, not the port's
	IndexerIDs []int // optional — restrict to specific configured indexers; empty means "all enabled"
}

// Category is one release's indexer category, as reported by the backend.
type Category struct {
	ID   int
	Name string
}

// IndexerRelease is a provider-neutral search result. Deliberately named
// IndexerRelease, not Release — ports.Release already names MusicBrainz's
// own DTO in internal/ports/musicbrainz.go, and reusing it here would
// collide two genuinely different shapes in one package.
type IndexerRelease struct {
	GUID        string
	Title       string
	IndexerName string
	Size        int64
	Protocol    Protocol
	PublishDate time.Time
	Seeders     int
	Leechers    int
	DownloadURL string
	MagnetURL   string
	InfoURL     string
	InfoHash    string
	Categories  []Category
}
