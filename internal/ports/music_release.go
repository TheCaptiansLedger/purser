package ports

import (
	"context"
	"purser/internal/domain/music"
)

// MusicReleaseRepository is the persistence port for music.Release.
// GetByMBID/GetByBarcode and the by-Group/by-Entry filtered lookups are
// added by later sub-issues in the Music Release API epic, extending this
// same interface — List takes no filter args yet. See
// docs/adr/0021-music-domain-model.md.
type MusicReleaseRepository interface {
	Create(ctx context.Context, r *music.Release) error
	Get(ctx context.Context, id string) (*music.Release, error)
	Update(ctx context.Context, r *music.Release) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pageSize int, pageToken string) (releases []*music.Release, nextPageToken string, err error)
}
