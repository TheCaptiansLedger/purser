package ports

import (
	"context"
	"errors"
	"purser/internal/domain"
)

// ErrNotSupported is returned by MetadataSource adapters for in-method capability
// guards (e.g. a method that accepts any ContentType but only handles music).
var ErrNotSupported = errors.New("not supported")

// ErrNotFound is returned by MetadataSource adapters when a lookup finds no match
// (as opposed to an error occurring during the lookup).
var ErrNotFound = errors.New("not found")

// MetadataSource is the base identity and routing contract every adapter must satisfy.
// Sub-interfaces below are optional capabilities checked via type assertion by the aggregator.
type MetadataSource interface {
	Name() string
	ContentTypes() []domain.ContentType
	ImagePriority() int
}

// StudioSearchSource is implemented by sources that support text search for
// studios or artists. A source may implement this without also implementing
// PeopleSearchSource or ItemSearchSource.
type StudioSearchSource interface {
	SearchStudios(ctx context.Context, query string, limit int) ([]*domain.ExternalStudio, error)
}

// PeopleSearchSource is implemented by sources that support text search for
// people or performers.
type PeopleSearchSource interface {
	SearchPeople(ctx context.Context, query string, limit int) ([]*domain.ExternalPerson, error)
}

// PersonRoleSource is implemented by sources that cover a specific set of
// person roles. The aggregator uses this to narrow fan-out when a role filter
// is provided: only sources that declare the requested role are queried.
// Sources that do not implement PersonRoleSource are always included so that
// sources with no declared role coverage act as a catch-all.
type PersonRoleSource interface {
	PersonRoles() []domain.PersonRole
}

// ItemSearchSource is implemented by sources that support text search for
// items, scenes, or recordings.
type ItemSearchSource interface {
	SearchItems(ctx context.Context, contentType domain.ContentType, query string, limit int) ([]*domain.ExternalItem, error)
}

// HashLookupSource is implemented by sources that can identify content by file
// hash (OSHash, MD5, etc.). Currently only stashdb implements this.
type HashLookupSource interface {
	FindByHash(ctx context.Context, hash string) (*domain.ExternalItem, error)
}

// ExternalIDSource is implemented by sources that hydrate a full entity from
// an external database ID. All current adapters implement this.
type ExternalIDSource interface {
	FindByExternalID(ctx context.Context, contentType domain.ContentType, id string) (*domain.ExternalItem, error)
}

// EntryContentSource is implemented by sources that page through the direct
// children of a library entry (scenes for a studio, release-groups for an artist).
// Returns groups (deep hierarchy) or items (flat hierarchy), never both.
type EntryContentSource interface {
	FetchEntryContent(ctx context.Context, contentType domain.ContentType, externalID string, page, perPage int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error)
}

// GroupContentSource is implemented by sources that page through items within
// a group (tracks within an album, episodes within a season).
type GroupContentSource interface {
	FetchGroupContent(ctx context.Context, contentType domain.ContentType, groupExternalID string, page, perPage int) ([]*domain.ExternalItem, int, error)
}

// EntryPeopleSource is implemented by sources that model people at the entry level
// (band members for a music artist). Currently only MusicBrainz implements this.
type EntryPeopleSource interface {
	FetchEntryPeople(ctx context.Context, externalID string) ([]*domain.ExternalPerson, error)
}

// GroupImageSource is implemented by sources requiring both a parent entity ID
// and a group ID to fetch group-level images. Currently only fanart implements this.
type GroupImageSource interface {
	FindGroupImages(ctx context.Context, contentType domain.ContentType, parentExtID, groupExtID string) (*domain.ExternalItem, error)
}

// PersonImageSource is implemented by sources that provide person-level hero images.
// fanart and theaudiodb implement this.
type PersonImageSource interface {
	FetchPersonImage(ctx context.Context, extID string) (*domain.ExternalImage, error)
}

// StudioThumbSource is implemented by sources that can return a studio or artist
// thumbnail URL by external ID. Used to enrich search results that have an MBID
// but no image after the initial text-search fan-out.
type StudioThumbSource interface {
	FetchStudioThumb(ctx context.Context, externalID string) (string, error)
}
