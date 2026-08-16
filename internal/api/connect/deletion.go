package apiconnect

import (
	"context"
	"purser/internal/domain"
)

// entityDeletionService is the narrow interface shared by every entity
// handler's Delete/GetXxxDeletionImpact methods — backed by that entity's
// composing service.XxxDeletionService, per
// docs/adr/0015-deletion-impact-and-composing-services.md. Declared once,
// not per-handler, since every composing deletion service exposes the
// same two-method shape regardless of entity.
type entityDeletionService interface {
	GetDeletionImpact(ctx context.Context, id string) (*domain.DeletionImpact, error)
	Delete(ctx context.Context, id string, cascade bool) error
}

// bulkDeletionService is entityDeletionService plus DeleteBatch — the
// interface ItemHandler, TagHandler, GroupHandler, and LibraryEntryHandler
// depend on, since those are the entities with a bulk-delete endpoint per
// docs/adr/0016-bulk-operations.md. Person stays on the narrower
// entityDeletionService.
type bulkDeletionService interface {
	entityDeletionService
	DeleteBatch(ctx context.Context, ids []string, cascade bool) error
}
