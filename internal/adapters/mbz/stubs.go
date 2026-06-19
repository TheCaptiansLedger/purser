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

// FetchGroupContent returns ErrNotSupported — implemented in a follow-up issue.
func (a *Adapter) FetchGroupContent(_ context.Context, _ string, _, _ int) ([]*domain.ExternalItem, int, error) {
	return nil, 0, ports.ErrNotSupported
}
