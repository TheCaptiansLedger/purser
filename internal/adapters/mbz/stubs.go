package mbz

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// FindByHash returns ErrNotSupported — MusicBrainz has no hash-based lookup.
func (a *Adapter) FindByHash(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

// FindGroupImages returns ErrNotSupported — MusicBrainz has no group-level image concept.
func (a *Adapter) FindGroupImages(_ context.Context, _ domain.ContentType, _, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

// FetchPersonImage returns ErrNotSupported — MusicBrainz does not serve artist images.
func (a *Adapter) FetchPersonImage(_ context.Context, _ string) (*domain.ExternalImage, error) {
	return nil, ports.ErrNotSupported
}
