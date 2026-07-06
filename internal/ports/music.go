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
