package ports

import (
	"context"
	"purser/internal/domain"
)

// UnmatchedFileRepository is the persistence port for
// domain.UnmatchedFile. Create/Get/List/Update/GetByHash for now — Resolve
// is added by a later sub-issue of the Common Scan Pipeline epic as this
// interface widens further. See docs/adr/0024-pipeline-core.md.
type UnmatchedFileRepository interface {
	Create(ctx context.Context, u *domain.UnmatchedFile) error
	Get(ctx context.Context, id string) (*domain.UnmatchedFile, error)

	// Update replaces the record stored under u's own ID — used by the
	// "already known" short-circuit to update Path when a queued file
	// moved on disk. Returns ports.ErrNotFound if none exists.
	Update(ctx context.Context, u *domain.UnmatchedFile) error

	// List returns a page of UnmatchedFiles, optionally filtered by status
	// (empty status means no filter), using opaque cursor pagination.
	List(ctx context.Context, status domain.UnmatchedFileStatus, pageSize int, pageToken string) ([]*domain.UnmatchedFile, string, error)

	// GetByHash returns the UnmatchedFile matching any of
	// oshash/sha1/md5/sha512 — checked in that order, first non-empty
	// match wins. An empty input is skipped rather than matched. Returns
	// ports.ErrNotFound if none of the non-empty inputs match. This is the
	// "already known" short-circuit's UnmatchedFile-side lookup — see
	// docs/adr/0024-pipeline-core.md.
	GetByHash(ctx context.Context, oshash, sha1, md5, sha512 string) (*domain.UnmatchedFile, error)
}
