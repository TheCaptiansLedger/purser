package apiconnect_test

import (
	"context"
	"purser/internal/domain"
)

// fakeEntityDeletionService is a test double for the shared
// entityDeletionService interface every entity handler's Delete/
// GetXxxDeletionImpact methods depend on — one fake, reused across every
// entity's handler tests, since the interface itself is shared. See
// docs/adr/0015-deletion-impact-and-composing-services.md.
type fakeEntityDeletionService struct {
	impact     *domain.DeletionImpact
	impactErr  error
	deleteErr  error
	gotID      string
	gotCascade bool
}

func newFakeEntityDeletionService() *fakeEntityDeletionService {
	return &fakeEntityDeletionService{}
}

func (f *fakeEntityDeletionService) GetDeletionImpact(_ context.Context, id string) (*domain.DeletionImpact, error) {
	f.gotID = id
	if f.impactErr != nil {
		return nil, f.impactErr
	}
	return f.impact, nil
}

func (f *fakeEntityDeletionService) Delete(_ context.Context, id string, cascade bool) error {
	f.gotID, f.gotCascade = id, cascade
	return f.deleteErr
}

// fakeBulkDeletionService is a test double for the shared
// bulkDeletionService interface ItemHandler/TagHandler depend on — see
// docs/adr/0016-bulk-operations.md.
type fakeBulkDeletionService struct {
	fakeEntityDeletionService
	deleteBatchErr  error
	gotBatchIDs     []string
	gotBatchCascade bool
}

func newFakeBulkDeletionService() *fakeBulkDeletionService {
	return &fakeBulkDeletionService{}
}

func (f *fakeBulkDeletionService) DeleteBatch(_ context.Context, ids []string, cascade bool) error {
	f.gotBatchIDs, f.gotBatchCascade = ids, cascade
	return f.deleteBatchErr
}
