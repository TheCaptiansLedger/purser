package ports_test

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// stubMetadataSource verifies that a hand-rolled implementation satisfies MetadataSource.
type stubMetadataSource struct{}

func (s *stubMetadataSource) Name() string                       { return "stub" }
func (s *stubMetadataSource) ContentTypes() []domain.ContentType { return nil }

func (s *stubMetadataSource) SearchStudios(_ context.Context, _ string, _ int) ([]*domain.ExternalStudio, error) {
	return nil, nil
}

func (s *stubMetadataSource) SearchPeople(_ context.Context, _ string, _ int) ([]*domain.ExternalPerson, error) {
	return nil, nil
}

func (s *stubMetadataSource) SearchItems(_ context.Context, _ domain.ContentType, _ string, _ int) ([]*domain.ExternalItem, error) {
	return nil, nil
}

func (s *stubMetadataSource) FindByHash(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

func (s *stubMetadataSource) FindByExternalID(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotFound
}

func (s *stubMetadataSource) FetchEntryContent(_ context.Context, _ string, _, _ int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	return nil, nil, 0, ports.ErrNotSupported
}

func (s *stubMetadataSource) FetchGroupContent(_ context.Context, _ string, _, _ int) ([]*domain.ExternalItem, int, error) {
	return nil, 0, ports.ErrNotSupported
}

func (s *stubMetadataSource) FetchEntryPeople(_ context.Context, _ string) ([]*domain.ExternalPerson, error) {
	return nil, ports.ErrNotSupported
}

// Compile-time interface satisfaction check.
var _ ports.MetadataSource = (*stubMetadataSource)(nil)
