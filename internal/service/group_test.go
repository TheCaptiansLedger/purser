package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeGroupRepository struct {
	byID map[string]*domain.Group
}

func newFakeGroupRepository() *fakeGroupRepository {
	return &fakeGroupRepository{byID: make(map[string]*domain.Group)}
}

func (f *fakeGroupRepository) Create(_ context.Context, g *domain.Group) error {
	if _, exists := f.byID[g.ID]; exists {
		return ports.ErrConflict
	}
	stored := *g
	f.byID[g.ID] = &stored
	return nil
}

func (f *fakeGroupRepository) Get(_ context.Context, id string) (*domain.Group, error) {
	g, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *g
	return &stored, nil
}

func (f *fakeGroupRepository) Update(_ context.Context, g *domain.Group) error {
	if _, ok := f.byID[g.ID]; !ok {
		return ports.ErrNotFound
	}
	stored := *g
	f.byID[g.ID] = &stored
	return nil
}

func (f *fakeGroupRepository) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeGroupRepository) List(_ context.Context, _ string, _ int, _ string) ([]*domain.Group, string, error) {
	groups := make([]*domain.Group, 0, len(f.byID))
	for _, g := range f.byID {
		stored := *g
		groups = append(groups, &stored)
	}
	return groups, "", nil
}

func validGroup(id string) *domain.Group {
	return &domain.Group{
		ID:             id,
		LibraryEntryID: "entry1",
		Title:          "Test Group",
		MonitorMode:    domain.MonitorModeNone,
	}
}

func TestGroupService_Create(t *testing.T) {
	t.Run("valid group is persisted", func(t *testing.T) {
		repo := newFakeGroupRepository()
		svc := service.NewGroupService(repo)

		got, err := svc.Create(context.Background(), validGroup("g1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.ID != "g1" {
			t.Fatalf("Create returned ID %q, want %q", got.ID, "g1")
		}
	})

	t.Run("invalid group is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeGroupRepository()
		svc := service.NewGroupService(repo)

		invalid := validGroup("g1")
		invalid.Title = ""

		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid group returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeGroupRepository()
		svc := service.NewGroupService(repo)

		if _, err := svc.Create(context.Background(), validGroup("g1")); err != nil {
			t.Fatalf("first Create returned error: %v", err)
		}
		if _, err := svc.Create(context.Background(), validGroup("g1")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("duplicate Create returned %v, want ErrConflict", err)
		}
	})
}

func TestGroupService_Get(t *testing.T) {
	repo := newFakeGroupRepository()
	svc := service.NewGroupService(repo)

	if _, err := svc.Create(context.Background(), validGroup("g1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "g1"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing group returned %v, want ErrNotFound", err)
	}
}

func TestGroupService_Update(t *testing.T) {
	t.Run("valid update is persisted", func(t *testing.T) {
		repo := newFakeGroupRepository()
		svc := service.NewGroupService(repo)

		if _, err := svc.Create(context.Background(), validGroup("g1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		updated := validGroup("g1")
		updated.Title = "Updated"
		if _, err := svc.Update(context.Background(), updated); err != nil {
			t.Fatalf("Update returned error: %v", err)
		}

		got, err := svc.Get(context.Background(), "g1")
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.Title != "Updated" {
			t.Fatalf("Get after Update returned Title %q, want %q", got.Title, "Updated")
		}
	})

	t.Run("invalid group is rejected", func(t *testing.T) {
		repo := newFakeGroupRepository()
		svc := service.NewGroupService(repo)

		if _, err := svc.Create(context.Background(), validGroup("g1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		invalid := validGroup("g1")
		invalid.Title = ""
		var verr *domain.ValidationError
		if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Update with invalid group returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("update of a missing group returns ErrNotFound", func(t *testing.T) {
		repo := newFakeGroupRepository()
		svc := service.NewGroupService(repo)

		if _, err := svc.Update(context.Background(), validGroup("missing")); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Update on missing group returned %v, want ErrNotFound", err)
		}
	})
}

func TestGroupService_Delete(t *testing.T) {
	repo := newFakeGroupRepository()
	svc := service.NewGroupService(repo)

	if _, err := svc.Create(context.Background(), validGroup("g1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(context.Background(), "g1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "g1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestGroupService_List(t *testing.T) {
	repo := newFakeGroupRepository()
	svc := service.NewGroupService(repo)

	for _, id := range []string{"g1", "g2"} {
		if _, err := svc.Create(context.Background(), validGroup(id)); err != nil {
			t.Fatalf("Create(%q) returned error: %v", id, err)
		}
	}

	groups, _, err := svc.List(context.Background(), "", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("List returned %d groups, want 2", len(groups))
	}
}
