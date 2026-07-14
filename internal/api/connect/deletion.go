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
