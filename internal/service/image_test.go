package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeImageRepository struct {
	byID map[string]*domain.Image
	// forceConflict makes the next Create return ports.ErrConflict without
	// touching byID — with server-generated IDs (see
	// docs/adr/0020-server-generated-kernel-entity-ids.md), two Creates
	// can no longer be forced to collide by reusing a literal ID.
	forceConflict bool
}

func newFakeImageRepository() *fakeImageRepository {
	return &fakeImageRepository{byID: make(map[string]*domain.Image)}
}

func (f *fakeImageRepository) Create(_ context.Context, img *domain.Image) error {
	if f.forceConflict {
		return ports.ErrConflict
	}
	if _, exists := f.byID[img.ID]; exists {
		return ports.ErrConflict
	}
	stored := *img
	f.byID[img.ID] = &stored
	return nil
}

func (f *fakeImageRepository) Get(_ context.Context, id string) (*domain.Image, error) {
	img, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *img
	return &stored, nil
}

func (f *fakeImageRepository) Update(_ context.Context, img *domain.Image) error {
	if _, ok := f.byID[img.ID]; !ok {
		return ports.ErrNotFound
	}
	stored := *img
	f.byID[img.ID] = &stored
	return nil
}

func (f *fakeImageRepository) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeImageRepository) List(_ context.Context, _, _ string, _ int, _ string) ([]*domain.Image, string, error) {
	images := make([]*domain.Image, 0, len(f.byID))
	for _, img := range f.byID {
		stored := *img
		images = append(images, &stored)
	}
	return images, "", nil
}

func validImage(id string) *domain.Image {
	return &domain.Image{ID: id, OwnerType: "person", OwnerID: "p1", ImageType: domain.ImageTypePoster, URL: "https://example.com/i.jpg"}
}

func TestImageService_Create(t *testing.T) {
	t.Run("valid image is persisted", func(t *testing.T) {
		repo := newFakeImageRepository()
		svc := service.NewImageService(repo)

		got, err := svc.Create(context.Background(), validImage("i1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.ID == "" || got.ID == "i1" {
			t.Fatalf("Create returned ID %q, want a server-generated one", got.ID)
		}
	})

	t.Run("invalid image is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeImageRepository()
		svc := service.NewImageService(repo)

		invalid := validImage("i1")
		invalid.URL = ""
		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid image returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeImageRepository()
		repo.forceConflict = true
		svc := service.NewImageService(repo)

		if _, err := svc.Create(context.Background(), validImage("i1")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("Create returned %v, want ErrConflict", err)
		}
	})
}

func TestImageService_Get(t *testing.T) {
	repo := newFakeImageRepository()
	svc := service.NewImageService(repo)

	created, err := svc.Create(context.Background(), validImage("i1"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), created.ID); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing image returned %v, want ErrNotFound", err)
	}
}

func TestImageService_Update(t *testing.T) {
	t.Run("valid update is persisted", func(t *testing.T) {
		repo := newFakeImageRepository()
		svc := service.NewImageService(repo)

		created, err := svc.Create(context.Background(), validImage("i1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		updated := validImage(created.ID)
		updated.Priority = 5
		if _, err := svc.Update(context.Background(), updated); err != nil {
			t.Fatalf("Update returned error: %v", err)
		}

		got, err := svc.Get(context.Background(), created.ID)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.Priority != 5 {
			t.Fatalf("Get after Update returned Priority %d, want 5", got.Priority)
		}
	})

	t.Run("update of a missing image returns ErrNotFound", func(t *testing.T) {
		repo := newFakeImageRepository()
		svc := service.NewImageService(repo)

		if _, err := svc.Update(context.Background(), validImage("missing")); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Update on missing image returned %v, want ErrNotFound", err)
		}
	})

	t.Run("invalid image is rejected", func(t *testing.T) {
		repo := newFakeImageRepository()
		svc := service.NewImageService(repo)

		if _, err := svc.Create(context.Background(), validImage("i1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		invalid := validImage("i1")
		invalid.URL = ""
		var verr *domain.ValidationError
		if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Update with invalid image returned %v, want *domain.ValidationError", err)
		}
	})
}

func TestImageService_Delete(t *testing.T) {
	repo := newFakeImageRepository()
	svc := service.NewImageService(repo)

	created, err := svc.Create(context.Background(), validImage("i1"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), created.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestImageService_List(t *testing.T) {
	repo := newFakeImageRepository()
	svc := service.NewImageService(repo)

	for _, id := range []string{"i1", "i2"} {
		if _, err := svc.Create(context.Background(), validImage(id)); err != nil {
			t.Fatalf("Create(%q) returned error: %v", id, err)
		}
	}

	images, _, err := svc.List(context.Background(), "", "", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("List returned %d images, want 2", len(images))
	}
}
