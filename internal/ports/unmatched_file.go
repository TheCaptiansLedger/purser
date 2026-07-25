package ports

import (
	"context"
	"purser/internal/domain"
)

// UnmatchedFileRepository is the persistence port for
// domain.UnmatchedFile. Create/Get only for now — List (e.g. filtered by
// Status), a hash-based lookup, and Update are added by later sub-issues
// of the Common Scan Pipeline epic as this interface widens; this issue's
// scan pipeline only needs to write and re-read one record at a time. See
// docs/adr/0024-pipeline-core.md.
type UnmatchedFileRepository interface {
	Create(ctx context.Context, u *domain.UnmatchedFile) error
	Get(ctx context.Context, id string) (*domain.UnmatchedFile, error)
}
