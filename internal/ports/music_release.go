package ports

import (
	"context"
	"purser/internal/domain/music"
)

// MusicReleaseRepository is the persistence port for music.Release. GetByMBID
// and GetByBarcode are unique point lookups; ListByGroup and ListByEntry are
// independent, paginated, indexed filters. See
// docs/adr/0021-music-domain-model.md.
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
}
