package theaudiodb

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// SearchPeople returns ErrNotSupported — TheAudioDB does not model individual people.
func (a *Adapter) SearchPeople(_ context.Context, _ string, _ int) ([]*domain.ExternalPerson, error) {
	return nil, ports.ErrNotSupported
}

// SearchItems returns ErrNotSupported — TheAudioDB has no track-level search; lookups are MBID-only.
func (a *Adapter) SearchItems(_ context.Context, _ domain.ContentType, _ string, _ int) ([]*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

// FindByHash returns ErrNotSupported — TheAudioDB has no hash-based lookup.
func (a *Adapter) FindByHash(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

// FetchEntryContent returns ErrNotSupported — TheAudioDB's free-tier discography
// endpoint does not return MBIDs or thumbnails; album covers are fetched per-album
// via FetchGroupContent instead.
func (a *Adapter) FetchEntryContent(_ context.Context, _ domain.ContentType, _ string, _, _ int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	return nil, nil, 0, ports.ErrNotSupported
}

// FetchEntryPeople returns ErrNotSupported — TheAudioDB does not model band membership.
func (a *Adapter) FetchEntryPeople(_ context.Context, _ string) ([]*domain.ExternalPerson, error) {
	return nil, ports.ErrNotSupported
}
