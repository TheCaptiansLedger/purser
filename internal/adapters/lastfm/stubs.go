package lastfm

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// FindByHash returns ErrNotSupported — Last.fm has no hash-based lookup.
func (a *Adapter) FindByHash(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

// SearchPeople returns ErrNotSupported — Last.fm artist.search returns bands and
// solo artists without distinction; callers should use SearchStudios instead.
func (a *Adapter) SearchPeople(_ context.Context, _ string, _ int) ([]*domain.ExternalPerson, error) {
	return nil, ports.ErrNotSupported
}

// SearchItems returns ErrNotSupported — Last.fm has no general track/album search.
func (a *Adapter) SearchItems(_ context.Context, _ domain.ContentType, _ string, _ int) ([]*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

// FindByExternalID returns ErrNotSupported — implemented in issue #88.
func (a *Adapter) FindByExternalID(_ context.Context, _ domain.ContentType, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

// FetchEntryContent returns ErrNotSupported — implemented in issue #89.
func (a *Adapter) FetchEntryContent(_ context.Context, _ domain.ContentType, _ string, _, _ int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	return nil, nil, 0, ports.ErrNotSupported
}

// FetchGroupContent returns ErrNotSupported — implemented in issue #90.
func (a *Adapter) FetchGroupContent(_ context.Context, _ domain.ContentType, _ string, _, _ int) ([]*domain.ExternalItem, int, error) {
	return nil, 0, ports.ErrNotSupported
}

// FetchEntryPeople returns ErrNotSupported — Last.fm does not model band members.
func (a *Adapter) FetchEntryPeople(_ context.Context, _ string) ([]*domain.ExternalPerson, error) {
	return nil, ports.ErrNotSupported
}
