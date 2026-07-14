package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeItemRepository struct {
	byID map[string]*domain.Item
}

func newFakeItemRepository() *fakeItemRepository {
	return &fakeItemRepository{byID: make(map[string]*domain.Item)}
}

func (f *fakeItemRepository) Create(_ context.Context, i *domain.Item) error {
	if _, exists := f.byID[i.ID]; exists {
		return ports.ErrConflict
	}
	stored := *i
	f.byID[i.ID] = &stored
	return nil
}

func (f *fakeItemRepository) Get(_ context.Context, id string) (*domain.Item, error) {
	i, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *i
	return &stored, nil
}

func (f *fakeItemRepository) Update(_ context.Context, i *domain.Item) error {
	if _, ok := f.byID[i.ID]; !ok {
		return ports.ErrNotFound
	}
	stored := *i
	f.byID[i.ID] = &stored
	return nil
}

func (f *fakeItemRepository) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeItemRepository) List(_ context.Context, _, _, _ string, _ int, _ string) ([]*domain.Item, string, error) {
	items := make([]*domain.Item, 0, len(f.byID))
	for _, i := range f.byID {
		stored := *i
		items = append(items, &stored)
	}
	return items, "", nil
}

func (f *fakeItemRepository) DeleteBatch(_ context.Context, ids []string) error {
	for _, id := range ids {
		if _, ok := f.byID[id]; !ok {
			return ports.ErrNotFound
		}
	}
	for _, id := range ids {
		delete(f.byID, id)
	}
	return nil
}

func validItem(id string) *domain.Item {
	return &domain.Item{
		ID:             id,
		ContentType:    domain.ContentTypeAdult,
		LibraryEntryID: "entry1",
		Title:          "Test Item",
		Status:         domain.ItemStatusWanted,
	}
}

func TestItemService_Create(t *testing.T) {
	t.Run("valid item is persisted", func(t *testing.T) {
		repo := newFakeItemRepository()
		svc := service.NewItemService(repo)

		got, err := svc.Create(context.Background(), validItem("i1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.ID != "i1" {
			t.Fatalf("Create returned ID %q, want %q", got.ID, "i1")
		}
	})

	t.Run("invalid item is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeItemRepository()
		svc := service.NewItemService(repo)

		invalid := validItem("i1")
		invalid.Title = ""

		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid item returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeItemRepository()
		svc := service.NewItemService(repo)

		if _, err := svc.Create(context.Background(), validItem("i1")); err != nil {
			t.Fatalf("first Create returned error: %v", err)
		}
		if _, err := svc.Create(context.Background(), validItem("i1")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("duplicate Create returned %v, want ErrConflict", err)
		}
	})
}

func TestItemService_Get(t *testing.T) {
	repo := newFakeItemRepository()
	svc := service.NewItemService(repo)

	if _, err := svc.Create(context.Background(), validItem("i1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "i1"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing item returned %v, want ErrNotFound", err)
	}
}

func TestItemService_Update(t *testing.T) {
	t.Run("valid update is persisted", func(t *testing.T) {
		repo := newFakeItemRepository()
		svc := service.NewItemService(repo)

		if _, err := svc.Create(context.Background(), validItem("i1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		updated := validItem("i1")
		updated.Title = "Updated"
		if _, err := svc.Update(context.Background(), updated); err != nil {
			t.Fatalf("Update returned error: %v", err)
		}

		got, err := svc.Get(context.Background(), "i1")
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.Title != "Updated" {
			t.Fatalf("Get after Update returned Title %q, want %q", got.Title, "Updated")
		}
	})

	t.Run("invalid item is rejected", func(t *testing.T) {
		repo := newFakeItemRepository()
		svc := service.NewItemService(repo)

		if _, err := svc.Create(context.Background(), validItem("i1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		invalid := validItem("i1")
		invalid.Title = ""
		var verr *domain.ValidationError
		if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Update with invalid item returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("update of a missing item returns ErrNotFound", func(t *testing.T) {
		repo := newFakeItemRepository()
		svc := service.NewItemService(repo)

		if _, err := svc.Update(context.Background(), validItem("missing")); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Update on missing item returned %v, want ErrNotFound", err)
		}
	})
}

func TestItemService_Delete(t *testing.T) {
	repo := newFakeItemRepository()
	svc := service.NewItemService(repo)

	if _, err := svc.Create(context.Background(), validItem("i1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(context.Background(), "i1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "i1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestItemService_List(t *testing.T) {
	repo := newFakeItemRepository()
	svc := service.NewItemService(repo)

	for _, id := range []string{"i1", "i2"} {
		if _, err := svc.Create(context.Background(), validItem(id)); err != nil {
			t.Fatalf("Create(%q) returned error: %v", id, err)
		}
	}

	items, _, err := svc.List(context.Background(), "", "", "", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("List returned %d items, want 2", len(items))
	}
}
