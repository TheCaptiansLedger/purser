package ports

import (
	"context"
	"purser/internal/domain"
)

// UnmatchedFileRepository is the persistence port for
// domain.UnmatchedFile. Create/Get/List for now — a hash-based lookup and
// Update are added by later sub-issues of the Common Scan Pipeline epic as
// this interface widens further. See docs/adr/0024-pipeline-core.md.
type UnmatchedFileRepository interface {
	Create(ctx context.Context, u *domain.UnmatchedFile) error
	Get(ctx context.Context, id string) (*domain.UnmatchedFile, error)

	// List returns a page of UnmatchedFiles, optionally filtered by status
	// (empty status means no filter), using opaque cursor pagination.
	List(ctx context.Context, status domain.UnmatchedFileStatus, pageSize int, pageToken string) ([]*domain.UnmatchedFile, string, error)
}
