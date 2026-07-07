package ports

import (
	"context"
	"purser/internal/domain"
)

// MusicReleaseRepository manages persistence for MusicRelease records.
type MusicReleaseRepository interface {
	Get(ctx context.Context, id string) (*domain.MusicRelease, error)
	GetByMBID(ctx context.Context, mbid string) (*domain.MusicRelease, error)
	GetByBarcode(ctx context.Context, barcode string) (*domain.MusicRelease, error)
	ListByGroup(ctx context.Context, groupID string) ([]*domain.MusicRelease, error)
	ListByEntry(ctx context.Context, entryID string) ([]*domain.MusicRelease, error)
	ListTracksByRelease(ctx context.Context, releaseID string) ([]*domain.Item, error)
	Save(ctx context.Context, r *domain.MusicRelease) error
	Delete(ctx context.Context, id string) error
}

// MusicScanGroupRepository manages persistence for MusicScanGroup queue entries.
type MusicScanGroupRepository interface {
	Get(ctx context.Context, id string) (*domain.MusicScanGroup, error)
	List(ctx context.Context, status domain.UnmatchedStatus) ([]*domain.MusicScanGroup, error)
	Save(ctx context.Context, g *domain.MusicScanGroup) error
	Delete(ctx context.Context, id string) error
}

// AlbumImporter imports an identified release when the album identifier's overall
// confidence meets the auto-import threshold. Implementations create the Release,
// its Tracks, MediaFiles, and write MBZ IDs back to file tags. The album
// identifier depends only on this seam so that the identification and import
// concerns stay decoupled — the concrete importer is supplied by the music
// import service.
type AlbumImporter interface {
	ImportRelease(ctx context.Context, candidate domain.MusicReleaseCandidate, group ScannedFileGroup) error
}
