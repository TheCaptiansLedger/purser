package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeTagRepository struct {
	byID map[string]*domain.Tag
}

func newFakeTagRepository() *fakeTagRepository {
	return &fakeTagRepository{byID: make(map[string]*domain.Tag)}
}

func (f *fakeTagRepository) Create(_ context.Context, t *domain.Tag) error {
	if _, exists := f.byID[t.ID]; exists {
		return ports.ErrConflict
	}
	stored := *t
	f.byID[t.ID] = &stored
	return nil
}

func (f *fakeTagRepository) Get(_ context.Context, id string) (*domain.Tag, error) {
	t, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *t
	return &stored, nil
}

func (f *fakeTagRepository) Update(_ context.Context, t *domain.Tag) error {
	if _, ok := f.byID[t.ID]; !ok {
		return ports.ErrNotFound
	}
	stored := *t
	f.byID[t.ID] = &stored
	return nil
}

func (f *fakeTagRepository) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeTagRepository) List(_ context.Context, _ int, _ string) ([]*domain.Tag, string, error) {
	tags := make([]*domain.Tag, 0, len(f.byID))
	for _, t := range f.byID {
		stored := *t
		tags = append(tags, &stored)
	}
	return tags, "", nil
}

func validTag(id string) *domain.Tag {
	return &domain.Tag{ID: id, Key: "genre", Value: "action", Scope: domain.TagScopeMetadata}
}

func TestTagService_Create(t *testing.T) {
	t.Run("valid tag is persisted", func(t *testing.T) {
		repo := newFakeTagRepository()
		svc := service.NewTagService(repo)

		got, err := svc.Create(context.Background(), validTag("t1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.ID != "t1" {
			t.Fatalf("Create returned ID %q, want %q", got.ID, "t1")
		}
	})

	t.Run("invalid tag is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeTagRepository()
		svc := service.NewTagService(repo)

		invalid := validTag("t1")
		invalid.Value = ""

		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid tag returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeTagRepository()
		svc := service.NewTagService(repo)

		if _, err := svc.Create(context.Background(), validTag("t1")); err != nil {
			t.Fatalf("first Create returned error: %v", err)
		}
		if _, err := svc.Create(context.Background(), validTag("t1")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("duplicate Create returned %v, want ErrConflict", err)
		}
	})
}

func TestTagService_Get(t *testing.T) {
	repo := newFakeTagRepository()
	svc := service.NewTagService(repo)

	if _, err := svc.Create(context.Background(), validTag("t1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "t1"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing tag returned %v, want ErrNotFound", err)
	}
}

func TestTagService_Update(t *testing.T) {
	t.Run("valid update is persisted", func(t *testing.T) {
		repo := newFakeTagRepository()
		svc := service.NewTagService(repo)

		if _, err := svc.Create(context.Background(), validTag("t1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		updated := validTag("t1")
		updated.Value = "gonzo"
		if _, err := svc.Update(context.Background(), updated); err != nil {
			t.Fatalf("Update returned error: %v", err)
		}

		got, err := svc.Get(context.Background(), "t1")
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.Value != "gonzo" {
			t.Fatalf("Get after Update returned Value %q, want %q", got.Value, "gonzo")
		}
	})

	t.Run("invalid tag is rejected", func(t *testing.T) {
		repo := newFakeTagRepository()
		svc := service.NewTagService(repo)

		if _, err := svc.Create(context.Background(), validTag("t1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		invalid := validTag("t1")
		invalid.Value = ""
		var verr *domain.ValidationError
		if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Update with invalid tag returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("update of a missing tag returns ErrNotFound", func(t *testing.T) {
		repo := newFakeTagRepository()
		svc := service.NewTagService(repo)

		if _, err := svc.Update(context.Background(), validTag("missing")); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Update on missing tag returned %v, want ErrNotFound", err)
		}
	})
}

func TestTagService_Delete(t *testing.T) {
	repo := newFakeTagRepository()
	svc := service.NewTagService(repo)

	if _, err := svc.Create(context.Background(), validTag("t1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(context.Background(), "t1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "t1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestTagService_List(t *testing.T) {
	repo := newFakeTagRepository()
	svc := service.NewTagService(repo)

	for _, id := range []string{"t1", "t2"} {
		if _, err := svc.Create(context.Background(), validTag(id)); err != nil {
			t.Fatalf("Create(%q) returned error: %v", id, err)
		}
	}

	tags, _, err := svc.List(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("List returned %d tags, want 2", len(tags))
	}
}
