package fanart

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// SearchStudios returns ErrNotSupported — fanart.tv has no studio/artist search.
func (a *Adapter) SearchStudios(_ context.Context, _ string, _ int) ([]*domain.ExternalStudio, error) {
	return nil, ports.ErrNotSupported
}

// SearchPeople returns ErrNotSupported — fanart.tv has no person search.
func (a *Adapter) SearchPeople(_ context.Context, _ string, _ int) ([]*domain.ExternalPerson, error) {
	return nil, ports.ErrNotSupported
}

// SearchItems returns ErrNotSupported — fanart.tv has no title-based search; lookups are ID-only.
func (a *Adapter) SearchItems(_ context.Context, _ domain.ContentType, _ string, _ int) ([]*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

// FindByHash returns ErrNotSupported — fanart.tv has no hash-based lookup.
func (a *Adapter) FindByHash(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

// FetchEntryPeople returns ErrNotSupported — fanart.tv does not model people.
func (a *Adapter) FetchEntryPeople(_ context.Context, _ string) ([]*domain.ExternalPerson, error) {
	return nil, ports.ErrNotSupported
}
