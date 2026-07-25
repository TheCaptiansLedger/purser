package ports

import (
	"context"
	"purser/internal/domain"
	"purser/internal/domain/music"
)

// MusicReleaseRepository is the persistence port for music.Release. GetByMBID
// and GetByBarcode are unique point lookups; ListByGroup and ListByEntry are
// independent, paginated, indexed filters. ListTracksByRelease answers the
// Track ↔ Release link: it is owned by this repository, not
// ports.ItemRepository, per docs/adr/0021-music-domain-model.md's "Track ↔
// Release linkage" section — ItemRepository/its List filter gain nothing new
// here.
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
}
