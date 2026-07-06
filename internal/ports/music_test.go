package ports_test

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// stubMusicReleaseRepository satisfies MusicReleaseRepository with no-op methods.
type stubMusicReleaseRepository struct{}

func (s *stubMusicReleaseRepository) Get(_ context.Context, _ string) (*domain.MusicRelease, error) {
	return nil, ports.ErrNotFound
}

func (s *stubMusicReleaseRepository) GetByMBID(_ context.Context, _ string) (*domain.MusicRelease, error) {
	return nil, ports.ErrNotFound
}

func (s *stubMusicReleaseRepository) GetByBarcode(_ context.Context, _ string) (*domain.MusicRelease, error) {
	return nil, ports.ErrNotFound
}

func (s *stubMusicReleaseRepository) ListByGroup(_ context.Context, _ string) ([]*domain.MusicRelease, error) {
	return nil, nil
}

func (s *stubMusicReleaseRepository) ListByEntry(_ context.Context, _ string) ([]*domain.MusicRelease, error) {
	return nil, nil
}

func (s *stubMusicReleaseRepository) ListTracksByRelease(_ context.Context, _ string) ([]*domain.Item, error) {
	return nil, nil
}

func (s *stubMusicReleaseRepository) Save(_ context.Context, _ *domain.MusicRelease) error {
	return nil
}
func (s *stubMusicReleaseRepository) Delete(_ context.Context, _ string) error { return nil }

// stubMusicScanGroupRepository satisfies MusicScanGroupRepository with no-op methods.
type stubMusicScanGroupRepository struct{}

func (s *stubMusicScanGroupRepository) Get(_ context.Context, _ string) (*domain.MusicScanGroup, error) {
	return nil, ports.ErrNotFound
}

func (s *stubMusicScanGroupRepository) List(_ context.Context, _ domain.UnmatchedStatus) ([]*domain.MusicScanGroup, error) {
	return nil, nil
}

func (s *stubMusicScanGroupRepository) Save(_ context.Context, _ *domain.MusicScanGroup) error {
	return nil
}
func (s *stubMusicScanGroupRepository) Delete(_ context.Context, _ string) error { return nil }

// Compile-time interface satisfaction checks.
var (
	_ ports.MusicReleaseRepository   = (*stubMusicReleaseRepository)(nil)
	_ ports.MusicScanGroupRepository = (*stubMusicScanGroupRepository)(nil)
)

// stubBaseSource satisfies only MetadataSource — not ReleaseGroupContentSource.
type stubBaseSource struct{}

func (s *stubBaseSource) Name() string                       { return "stub-base" }
func (s *stubBaseSource) ContentTypes() []domain.ContentType { return nil }
func (s *stubBaseSource) ImagePriority() int                 { return 0 }

var _ ports.MetadataSource = (*stubBaseSource)(nil)

func TestReleaseGroupContentSource_IsOptional(t *testing.T) {
	var src ports.MetadataSource = &stubBaseSource{}
	if _, ok := src.(ports.ReleaseGroupContentSource); ok {
		t.Fatal("stubBaseSource must not satisfy ReleaseGroupContentSource")
	}
}
