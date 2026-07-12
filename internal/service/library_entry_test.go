package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeLibraryEntryRepository struct {
	byID map[string]*domain.LibraryEntry
}

func newFakeLibraryEntryRepository() *fakeLibraryEntryRepository {
	return &fakeLibraryEntryRepository{byID: make(map[string]*domain.LibraryEntry)}
}

func (f *fakeLibraryEntryRepository) Create(_ context.Context, e *domain.LibraryEntry) error {
	if _, exists := f.byID[e.ID]; exists {
		return ports.ErrConflict
	}
	stored := *e
	f.byID[e.ID] = &stored
	return nil
}

func (f *fakeLibraryEntryRepository) Get(_ context.Context, id string) (*domain.LibraryEntry, error) {
	e, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *e
	return &stored, nil
}

func (f *fakeLibraryEntryRepository) Update(_ context.Context, e *domain.LibraryEntry) error {
	if _, ok := f.byID[e.ID]; !ok {
		return ports.ErrNotFound
	}
	stored := *e
	f.byID[e.ID] = &stored
	return nil
}

func (f *fakeLibraryEntryRepository) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeLibraryEntryRepository) List(_ context.Context, _ int, _ string) ([]*domain.LibraryEntry, string, error) {
	entries := make([]*domain.LibraryEntry, 0, len(f.byID))
	for _, e := range f.byID {
		stored := *e
		entries = append(entries, &stored)
	}
	return entries, "", nil
}

func validLibraryEntry(id string) *domain.LibraryEntry {
	return &domain.LibraryEntry{
		ID:          id,
		ContentType: domain.ContentTypeAdult,
		Kind:        domain.KindStudio,
		Name:        "Test Entry",
		MonitorMode: domain.MonitorModeNone,
	}
}

func TestLibraryEntryService_Create(t *testing.T) {
	t.Run("valid entry is persisted", func(t *testing.T) {
		repo := newFakeLibraryEntryRepository()
		svc := service.NewLibraryEntryService(repo)

		got, err := svc.Create(context.Background(), validLibraryEntry("e1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.ID != "e1" {
			t.Fatalf("Create returned ID %q, want %q", got.ID, "e1")
		}
	})

	t.Run("invalid entry is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeLibraryEntryRepository()
		svc := service.NewLibraryEntryService(repo)

		invalid := validLibraryEntry("e1")
		invalid.Name = ""

		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid entry returned %v, want *domain.ValidationError", err)
		}
		if _, err := repo.Get(context.Background(), "e1"); !errors.Is(err, ports.ErrNotFound) {
			t.Fatal("Create with invalid entry reached the repository")
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeLibraryEntryRepository()
		svc := service.NewLibraryEntryService(repo)

		if _, err := svc.Create(context.Background(), validLibraryEntry("e1")); err != nil {
			t.Fatalf("first Create returned error: %v", err)
		}
		if _, err := svc.Create(context.Background(), validLibraryEntry("e1")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("duplicate Create returned %v, want ErrConflict", err)
		}
	})
}

func TestLibraryEntryService_Get(t *testing.T) {
	repo := newFakeLibraryEntryRepository()
	svc := service.NewLibraryEntryService(repo)

	if _, err := svc.Create(context.Background(), validLibraryEntry("e1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "e1"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing entry returned %v, want ErrNotFound", err)
	}
}

func TestLibraryEntryService_Update(t *testing.T) {
	t.Run("valid update is persisted", func(t *testing.T) {
		repo := newFakeLibraryEntryRepository()
		svc := service.NewLibraryEntryService(repo)

		if _, err := svc.Create(context.Background(), validLibraryEntry("e1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		updated := validLibraryEntry("e1")
		updated.Name = "Updated"
		if _, err := svc.Update(context.Background(), updated); err != nil {
			t.Fatalf("Update returned error: %v", err)
		}

		got, err := svc.Get(context.Background(), "e1")
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.Name != "Updated" {
			t.Fatalf("Get after Update returned Name %q, want %q", got.Name, "Updated")
		}
	})

	t.Run("invalid entry is rejected", func(t *testing.T) {
		repo := newFakeLibraryEntryRepository()
		svc := service.NewLibraryEntryService(repo)

		if _, err := svc.Create(context.Background(), validLibraryEntry("e1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		invalid := validLibraryEntry("e1")
		invalid.Name = ""
		var verr *domain.ValidationError
		if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Update with invalid entry returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("update of a missing entry returns ErrNotFound", func(t *testing.T) {
		repo := newFakeLibraryEntryRepository()
		svc := service.NewLibraryEntryService(repo)

		if _, err := svc.Update(context.Background(), validLibraryEntry("missing")); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Update on missing entry returned %v, want ErrNotFound", err)
		}
	})
}

func TestLibraryEntryService_Delete(t *testing.T) {
	repo := newFakeLibraryEntryRepository()
	svc := service.NewLibraryEntryService(repo)

	if _, err := svc.Create(context.Background(), validLibraryEntry("e1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(context.Background(), "e1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "e1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestLibraryEntryService_List(t *testing.T) {
	repo := newFakeLibraryEntryRepository()
	svc := service.NewLibraryEntryService(repo)

	for _, id := range []string{"e1", "e2"} {
		if _, err := svc.Create(context.Background(), validLibraryEntry(id)); err != nil {
			t.Fatalf("Create(%q) returned error: %v", id, err)
		}
	}

	entries, _, err := svc.List(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("List returned %d entries, want 2", len(entries))
	}
}
