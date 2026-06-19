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
