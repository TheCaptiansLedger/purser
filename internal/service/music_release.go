package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/domain/music"
	"purser/internal/ports"
)

// MusicReleaseService orchestrates music.Release against a
// ports.MusicReleaseRepository. See GroupService for the conventions this
// follows — Music's first module-specific service, added with zero edits
// to any kernel service file.
type MusicReleaseService struct {
	repo ports.MusicReleaseRepository
}

// NewMusicReleaseService constructs a MusicReleaseService backed by repo.
func NewMusicReleaseService(repo ports.MusicReleaseRepository) *MusicReleaseService {
	return &MusicReleaseService{repo: repo}
}

// Create assigns r a server-generated ID (see
// docs/adr/0020-server-generated-kernel-entity-ids.md), validates it, and
// persists it.
func (s *MusicReleaseService) Create(ctx context.Context, r *music.Release) (*music.Release, error) {
	r.ID = domain.NewID()
	if err := r.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Get returns the Release with the given ID, or ports.ErrNotFound.
func (s *MusicReleaseService) Get(ctx context.Context, id string) (*music.Release, error) {
	return s.repo.Get(ctx, id)
}
