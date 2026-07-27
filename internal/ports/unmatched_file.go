package ports

import (
	"context"
	"purser/internal/domain"
)

// UnmatchedFileRepository is the persistence port for domain.UnmatchedFile.
// See docs/adr/0024-pipeline-core.md.
type UnmatchedFileRepository interface {
	Create(ctx context.Context, u *domain.UnmatchedFile) error
	Get(ctx context.Context, id string) (*domain.UnmatchedFile, error)

	// Delete removes the UnmatchedFile with the given id — used by
	// UnmatchedFileService.Resolve's match outcome, since a resolved
	// entry leaves the review queue once its MediaFile takes its place.
	// Returns ports.ErrNotFound if none exists.
	Delete(ctx context.Context, id string) error

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

	// ListByGroupKey returns every UnmatchedFile sharing groupKey — unlike
	// GetByHash (wants the first match, stops), this needs every row in
	// the group, so the implementation loops pages to completion rather
	// than assuming one page covers it. An unknown groupKey returns an
	// empty slice, not ports.ErrNotFound. See
	// docs/technical/pipeline-unmatchedfile-grouping.md.
	ListByGroupKey(ctx context.Context, groupKey string) ([]*domain.UnmatchedFile, error)

	// UpdateBatch replaces every record in us, re-indexed, as a single
	// atomic transaction — all succeed or none do. Returns
	// ports.ErrNotFound if any id in us doesn't already exist. See
	// docs/adr/0016-bulk-operations.md.
	UpdateBatch(ctx context.Context, us []*domain.UnmatchedFile) error

	// DeleteBatch removes every UnmatchedFile in ids as a single atomic
	// transaction — all succeed or none do. Used once a group's winning
	// candidate has been persisted: the whole group leaves the review
	// queue by deletion, the same "a matched entry leaves the queue by
	// deletion" behavior Resolve already has for a single file, applied to
	// a whole group at once. Returns ports.ErrNotFound if any id in ids
	// doesn't already exist. See docs/adr/0016-bulk-operations.md.
	DeleteBatch(ctx context.Context, ids []string) error
}
