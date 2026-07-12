package ports

import (
	"context"
	"purser/internal/domain/afterdark"
)

// PerformerProfileRepository is the persistence port for
// afterdark.PerformerProfile. Its identity is PersonID alone — no
// independent ID, same reasoning as ExternalIDRepository — so Get/Delete
// key on personID directly. This is AfterDark's only module-specific
// port; it imports domain/afterdark, never the reverse, keeping the
// dependency arrow pointed the same way ADR 0001 requires for kernel
// ports.
type PerformerProfileRepository interface {
	Create(ctx context.Context, p *afterdark.PerformerProfile) error
	Get(ctx context.Context, personID string) (*afterdark.PerformerProfile, error)
	Update(ctx context.Context, p *afterdark.PerformerProfile) error
	Delete(ctx context.Context, personID string) error
	List(ctx context.Context, pageSize int, pageToken string) (profiles []*afterdark.PerformerProfile, nextPageToken string, err error)
}
