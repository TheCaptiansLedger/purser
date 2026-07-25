package ports

import (
	"context"
	"purser/internal/domain"
)

// MediaFileRepository is the persistence port for domain.MediaFile. See
// PersonRepository for the pagination convention. itemID is an optional
// filter — an empty string means "no filter on this field."
type MediaFileRepository interface {
	Create(ctx context.Context, m *domain.MediaFile) error
	Get(ctx context.Context, id string) (*domain.MediaFile, error)
	Update(ctx context.Context, m *domain.MediaFile) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, itemID string, pageSize int, pageToken string) (mediaFiles []*domain.MediaFile, nextPageToken string, err error)

	// GetByHash returns the MediaFile matching any of oshash/sha1/md5/sha512
	// — checked in that order, first non-empty match wins. An empty input
	// is skipped rather than matched (it would otherwise match every
	// record that hasn't computed that hash type). Returns
	// ports.ErrNotFound if none of the non-empty inputs match. This is the
	// "already known" short-circuit's MediaFile-side lookup — see
	// docs/adr/0024-pipeline-core.md.
	GetByHash(ctx context.Context, oshash, sha1, md5, sha512 string) (*domain.MediaFile, error)
}
