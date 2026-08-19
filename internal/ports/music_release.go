package ports

import (
	"context"
	"purser/internal/domain"
	"purser/internal/domain/music"
)

// MusicReleaseRepository is the persistence port for music.Release.
// GetByBarcode is a plain (unenforced) point lookup; GetByMBID is a real
// unique lookup — Create is get-or-create on r.MBID whenever it's non-empty
// (a stub release with no MBID yet creates plainly, no uniqueness check):
// if a release with that MBID already exists, r is mutated in place to the
// pre-existing release and Create returns nil. See
// docs/technical/pipeline-music-persist.md's "MusicRelease's own
// reservation-document fix". ListByGroup and ListByEntry are independent,
// paginated, indexed filters. ListTracksByRelease answers the Track ↔
// Release link: it is owned by this repository, not ports.ItemRepository,
// per docs/adr/0021-music-domain-model.md's "Track ↔ Release linkage"
// section — ItemRepository/its List filter gain nothing new here.
type MusicReleaseRepository interface {
	Create(ctx context.Context, r *music.Release) error
	Get(ctx context.Context, id string) (*music.Release, error)
	GetByMBID(ctx context.Context, mbid string) (*music.Release, error)
	GetByBarcode(ctx context.Context, barcode string) (*music.Release, error)
	Update(ctx context.Context, r *music.Release) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pageSize int, pageToken string) (releases []*music.Release, nextPageToken string, err error)
	ListByGroup(ctx context.Context, groupID string, pageSize int, pageToken string) (releases []*music.Release, nextPageToken string, err error)
	ListByEntry(ctx context.Context, libraryEntryID string, pageSize int, pageToken string) (releases []*music.Release, nextPageToken string, err error)

	// ListTracksByRelease returns every Item(ContentType=music) whose
	// Metadata["release_id"] equals releaseID — the tracks on this
	// specific pressing/edition. Returns ports.ErrNotFound if releaseID
	// doesn't exist.
	ListTracksByRelease(ctx context.Context, releaseID string, pageSize int, pageToken string) (tracks []*domain.Item, nextPageToken string, err error)

	// CreateTrack persists track as a kernel Item, GroupID and
	// Metadata["release_id"] populated from releaseID — see this port's own
	// ListTracksByRelease doc comment for why this linkage lives here and
	// not on ports.ItemRepository. track.ID must already be set by the
	// caller (ports.MusicReleaseRepository callers assign IDs the same way
	// every other kernel Create does, per
	// docs/adr/0020-server-generated-kernel-entity-ids.md). Returns
	// ports.ErrNotFound if releaseID doesn't exist.
	CreateTrack(ctx context.Context, releaseID string, track *domain.Item) error
}
