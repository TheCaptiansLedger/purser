package metadata_test

import (
	"context"
	"fmt"
	"testing"

	"purser/internal/app/errs"
	"purser/internal/app/metadata"
	"purser/internal/domain"
	"purser/internal/ports"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubEntryRepo struct {
	data map[string]*domain.LibraryEntry
}

func newStubEntryRepo() *stubEntryRepo {
	return &stubEntryRepo{data: make(map[string]*domain.LibraryEntry)}
}

func (r *stubEntryRepo) Get(_ context.Context, id string) (*domain.LibraryEntry, error) {
	e, ok := r.data[id]
	if !ok {
		return nil, fmt.Errorf("not found: %w", errs.ErrNotFound)
	}
	return e, nil
}

func (r *stubEntryRepo) List(_ context.Context, _ ports.LibraryFilter) ([]*domain.LibraryEntry, int, error) {
	return nil, 0, nil
}

func (r *stubEntryRepo) Save(_ context.Context, e *domain.LibraryEntry) error {
	r.data[e.ID] = e
	return nil
}

func (r *stubEntryRepo) Delete(_ context.Context, id string) error {
	delete(r.data, id)
	return nil
}

type stubPersonRepo struct{}

func (r *stubPersonRepo) Get(_ context.Context, _ string) (*domain.Person, error) {
	return nil, fmt.Errorf("not found: %w", errs.ErrNotFound)
}
func (r *stubPersonRepo) List(_ context.Context, _ ports.PersonFilter) ([]*domain.Person, int, error) {
	return nil, 0, nil
}
func (r *stubPersonRepo) Save(_ context.Context, _ *domain.Person) error  { return nil }
func (r *stubPersonRepo) Delete(_ context.Context, _ string) error         { return nil }

// stubExternalIDRepo returns ErrNotFound for every lookup, simulating a fresh
// database where no external IDs have been imported yet.
type stubExternalIDRepo struct{}

func (r *stubExternalIDRepo) FindEntity(_ context.Context, _, _, _ string) (string, error) {
	return "", fmt.Errorf("not found: %w", errs.ErrNotFound)
}

func newService() *metadata.Service {
	return metadata.New(
		nil, // no metadata sources needed for import tests
		newStubEntryRepo(),
		&stubPersonRepo{},
		&stubExternalIDRepo{},
		"", // no media path — image fetching is skipped when empty
	)
}

// ── ImportStudio ──────────────────────────────────────────────────────────────

func TestImportStudio_DefaultMonitorMode(t *testing.T) {
	svc := newService()

	res, err := svc.ImportStudio(context.Background(), &metadata.ImportStudioRequest{
		Source:      domain.SourceStashDB,
		ExternalID:  "studio-1",
		Name:        "Acme Studios",
		ContentType: domain.ContentTypeAdult,
		Monitored:   false,
		// MonitorMode deliberately omitted — should default to latest
	})
	if err != nil {
		t.Fatalf("ImportStudio: %v", err)
	}
	if res.Studio.MonitorMode != domain.MonitorLatest {
		t.Errorf("MonitorMode = %q, want %q", res.Studio.MonitorMode, domain.MonitorLatest)
	}
}

func TestImportStudio_ExplicitMonitorMode(t *testing.T) {
	svc := newService()

	res, err := svc.ImportStudio(context.Background(), &metadata.ImportStudioRequest{
		Source:      domain.SourceStashDB,
		ExternalID:  "studio-2",
		Name:        "Full Collection Studios",
		ContentType: domain.ContentTypeAdult,
		MonitorMode: domain.MonitorAll,
	})
	if err != nil {
		t.Fatalf("ImportStudio: %v", err)
	}
	if res.Studio.MonitorMode != domain.MonitorAll {
		t.Errorf("MonitorMode = %q, want %q", res.Studio.MonitorMode, domain.MonitorAll)
	}
}

func TestImportStudio_Idempotent(t *testing.T) {
	entryRepo := newStubEntryRepo()
	svc := metadata.New(nil, entryRepo, &stubPersonRepo{}, &stubExternalIDRepo{}, "")

	req := &metadata.ImportStudioRequest{
		Source:      domain.SourceStashDB,
		ExternalID:  "studio-3",
		Name:        "Once Only",
		ContentType: domain.ContentTypeAdult,
	}

	res1, err := svc.ImportStudio(context.Background(), req)
	if err != nil {
		t.Fatalf("first ImportStudio: %v", err)
	}

	// Seed the external ID repo with the saved entry so the second call finds it.
	seededRepo := &seededExternalIDRepo{id: res1.Studio.ID}
	svc2 := metadata.New(nil, entryRepo, &stubPersonRepo{}, seededRepo, "")

	res2, err := svc2.ImportStudio(context.Background(), req)
	if err != nil {
		t.Fatalf("second ImportStudio: %v", err)
	}
	if res2.Studio.ID != res1.Studio.ID {
		t.Errorf("idempotent call returned different ID: %q vs %q", res2.Studio.ID, res1.Studio.ID)
	}
	if len(entryRepo.data) != 1 {
		t.Errorf("entry count = %d, want 1 (no duplicate created)", len(entryRepo.data))
	}
}

type seededExternalIDRepo struct{ id string }

func (r *seededExternalIDRepo) FindEntity(_ context.Context, _, _, _ string) (string, error) {
	return r.id, nil
}
