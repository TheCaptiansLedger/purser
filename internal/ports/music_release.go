package ports

import (
	"context"
	"purser/internal/domain/music"
)

// MusicReleaseRepository is the persistence port for music.Release. Only
// Create and Get are defined for this walking-skeleton pass — Update,
// Delete, and List (plus GetByMBID/GetByBarcode and the by-Group/by-Entry
// filtered lookups) are added by later sub-issues in the Music Release API
// epic, extending this same interface. See
// docs/adr/0021-music-domain-model.md.
type MusicReleaseRepository interface {
	Create(ctx context.Context, r *music.Release) error
	Get(ctx context.Context, id string) (*music.Release, error)
}
