package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// MusicReleaseDeletionService is Release's composing deletion service — the
// explicit multi-port exception
// docs/adr/0015-deletion-impact-and-composing-services.md carves out.
// internal/service/music_release.go stays single-port; this is genuinely
// the exception, not a reason to weaken that rule generally.
//
// Its only referrer is Item (a track), via Item.Metadata["release_id"].
// That link is not a required foreign key, so Delete always runs Unlink —
// clearing Metadata["release_id"] on every referencing track, leaving the
// track itself intact — the same "detach, don't destroy" pattern
// GroupDeletionService already uses for Item.GroupID. Cascade is accepted
// for API-shape consistency but has no effect here, per
// docs/adr/0021-music-domain-model.md's Services section.
type MusicReleaseDeletionService struct {
	releases ports.MusicReleaseRepository
	items    ports.ItemRepository
}

// NewMusicReleaseDeletionService constructs a MusicReleaseDeletionService
// backed by the given ports.
func NewMusicReleaseDeletionService(releases ports.MusicReleaseRepository, items ports.ItemRepository) *MusicReleaseDeletionService {
	return &MusicReleaseDeletionService{releases: releases, items: items}
}

// GetDeletionImpact returns what references the Release identified by id,
// or ports.ErrNotFound if the Release doesn't exist.
func (s *MusicReleaseDeletionService) GetDeletionImpact(ctx context.Context, id string) (*domain.DeletionImpact, error) {
	if _, err := s.releases.Get(ctx, id); err != nil {
		return nil, err
	}

	tracks, err := s.drainTracks(ctx, id)
	if err != nil {
		return nil, err
	}

	return &domain.DeletionImpact{
		Impacts: []domain.DeletionImpactRow{
			{Kind: "item", Label: "Tracks", Count: len(tracks)},
		},
	}, nil
}

// Delete removes the Release identified by id, clearing
// Metadata["release_id"] on every track that pointed at it (leaving the
// tracks themselves intact). Returns ports.ErrNotFound if the Release
// doesn't exist.
func (s *MusicReleaseDeletionService) Delete(ctx context.Context, id string, _ bool) error {
	if _, err := s.releases.Get(ctx, id); err != nil {
		return err
	}
	if err := s.detachTracks(ctx, id); err != nil {
		return err
	}
	return s.releases.Delete(ctx, id)
}

func (s *MusicReleaseDeletionService) detachTracks(ctx context.Context, releaseID string) error {
	tracks, err := s.drainTracks(ctx, releaseID)
	if err != nil {
		return err
	}
	for _, t := range tracks {
		delete(t.Metadata, "release_id")
		if err := s.items.Update(ctx, t); err != nil {
			return err
		}
	}
	return nil
}

func (s *MusicReleaseDeletionService) drainTracks(ctx context.Context, releaseID string) ([]*domain.Item, error) {
	var out []*domain.Item
	pageToken := ""
	for {
		rows, next, err := s.releases.ListTracksByRelease(ctx, releaseID, 100, pageToken)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
		if next == "" {
			break
		}
		pageToken = next
	}
	return out, nil
}
