package ports

import (
	"context"
	"errors"
	"purser/internal/domain"
)

// ErrNotSupported is returned by MetadataSource adapters for methods the source
// does not implement (e.g., a source that has no hash-lookup capability).
var ErrNotSupported = errors.New("not supported")

// ErrNotFound is returned by MetadataSource adapters when a lookup finds no match
// (as opposed to an error occurring during the lookup).
var ErrNotFound = errors.New("not found")

// MetadataSource is the port for external metadata databases.
// Adapters implement this for StashDB, TPDB, Stash, TMDB, TVDB, MusicBrainz, etc.
// The app service fans out to all enabled sources and merges the results.
//
// Methods that a source does not support must return ErrNotSupported.
type MetadataSource interface {
	// Name returns the source identifier, e.g. "stashdb", "tmdb".
	Name() string

	// ContentTypes returns the content types this source can serve.
	ContentTypes() []domain.ContentType

	// SearchStudios searches for studios or networks by name.
	// Used by the "Add Site / Add Network" flow.
	SearchStudios(ctx context.Context, query string, limit int) ([]*domain.ExternalStudio, error)

	// SearchPeople searches for performers, artists, authors, or cast by name.
	// Used by the "Add Performer" flow.
	SearchPeople(ctx context.Context, query string, limit int) ([]*domain.ExternalPerson, error)

	// SearchItems searches for scenes, movies, episodes, tracks, or books by title.
	// contentType narrows results when the source serves multiple types.
	SearchItems(ctx context.Context, contentType domain.ContentType, query string, limit int) ([]*domain.ExternalItem, error)

	// FindByHash identifies a scene or item by file hash (OSHash or MD5).
	// Used for automatic file identification; sources without hash lookup return ErrNotSupported.
	FindByHash(ctx context.Context, hash string) (*domain.ExternalItem, error)

	// FindByExternalID fetches a fully-detailed item by its ID in this source's database.
	// Used to hydrate a search candidate before the metadata edit screen.
	FindByExternalID(ctx context.Context, id string) (*domain.ExternalItem, error)

	// FetchEntryContent fetches the direct children of a library entry, paginated.
	// For flat hierarchies (adult/jav studios, book series) groups is nil and items
	// contains the leaf content directly.
	// For deep hierarchies (TV shows, music artists) items is nil and groups contains
	// the intermediate layer (seasons, albums); call FetchGroupContent for each group
	// to retrieve its items.
	// Returns ErrNotSupported for content types the source does not handle.
	FetchEntryContent(ctx context.Context, externalID string, page, perPage int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error)

	// FetchGroupContent fetches items within a group (season→episodes, album→tracks),
	// paginated. Returns ErrNotSupported for sources or content types without this
	// level of hierarchy.
	FetchGroupContent(ctx context.Context, groupExternalID string, page, perPage int) ([]*domain.ExternalItem, int, error)

	// FetchEntryPeople returns people directly associated with a library entry
	// (e.g., band members for a music artist). Returns ErrNotSupported for sources
	// that do not model entry-level people.
	FetchEntryPeople(ctx context.Context, externalID string) ([]*domain.ExternalPerson, error)
}
