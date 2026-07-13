package ports

import (
	"context"
	"purser/internal/domain"
)

// LibraryEntryRepository is the persistence port for domain.LibraryEntry.
// It knows nothing about any other kernel entity, any transport, or any
// metadata provider — per docs/adr/0001-hexagonal-architecture.md.
//
// List uses opaque cursor pagination — see ports.PersonRepository for the
// convention. kind and parentID are independent, optional filters — an
// empty string means "no filter on this field." parentID filters on
// LibraryEntry's self-referential hierarchy (e.g. every Studio under a
// given Network); an empty parentID does not mean "top-level only," it
// means unfiltered on that field, same as every other filtered port in
// this codebase.
type LibraryEntryRepository interface {
	Create(ctx context.Context, entry *domain.LibraryEntry) error
	Get(ctx context.Context, id string) (*domain.LibraryEntry, error)
	Update(ctx context.Context, entry *domain.LibraryEntry) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, kind domain.Kind, parentID string, pageSize int, pageToken string) (entries []*domain.LibraryEntry, nextPageToken string, err error)
}
