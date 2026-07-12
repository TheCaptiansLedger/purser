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
// convention.
type LibraryEntryRepository interface {
	Create(ctx context.Context, entry *domain.LibraryEntry) error
	Get(ctx context.Context, id string) (*domain.LibraryEntry, error)
	Update(ctx context.Context, entry *domain.LibraryEntry) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, pageSize int, pageToken string) (entries []*domain.LibraryEntry, nextPageToken string, err error)
}
