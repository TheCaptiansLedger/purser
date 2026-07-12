package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeMediaFileRepository struct {
	byID map[string]*domain.MediaFile
}

func newFakeMediaFileRepository() *fakeMediaFileRepository {
	return &fakeMediaFileRepository{byID: make(map[string]*domain.MediaFile)}
}

func (f *fakeMediaFileRepository) Create(_ context.Context, m *domain.MediaFile) error {
	if _, exists := f.byID[m.ID]; exists {
		return ports.ErrConflict
	}
	stored := *m
	f.byID[m.ID] = &stored
	return nil
}

func (f *fakeMediaFileRepository) Get(_ context.Context, id string) (*domain.MediaFile, error) {
	m, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *m
	return &stored, nil
}

func (f *fakeMediaFileRepository) Update(_ context.Context, m *domain.MediaFile) error {
	if _, ok := f.byID[m.ID]; !ok {
		return ports.ErrNotFound
	}
	stored := *m
	f.byID[m.ID] = &stored
	return nil
}

func (f *fakeMediaFileRepository) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeMediaFileRepository) List(_ context.Context, _ int, _ string) ([]*domain.MediaFile, string, error) {
	files := make([]*domain.MediaFile, 0, len(f.byID))
	for _, m := range f.byID {
		stored := *m
		files = append(files, &stored)
	}
	return files, "", nil
}

func validMediaFile(id string) *domain.MediaFile {
	return &domain.MediaFile{ID: id, ItemID: "item1", Path: "/media/test.mkv"}
}

func TestMediaFileService_Create(t *testing.T) {
	t.Run("valid media file is persisted", func(t *testing.T) {
		repo := newFakeMediaFileRepository()
		svc := service.NewMediaFileService(repo)

		got, err := svc.Create(context.Background(), validMediaFile("m1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.ID != "m1" {
			t.Fatalf("Create returned ID %q, want %q", got.ID, "m1")
		}
	})

	t.Run("invalid media file is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeMediaFileRepository()
		svc := service.NewMediaFileService(repo)

		invalid := validMediaFile("m1")
		invalid.Path = ""
		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid media file returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeMediaFileRepository()
		svc := service.NewMediaFileService(repo)

		if _, err := svc.Create(context.Background(), validMediaFile("m1")); err != nil {
			t.Fatalf("first Create returned error: %v", err)
		}
		if _, err := svc.Create(context.Background(), validMediaFile("m1")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("duplicate Create returned %v, want ErrConflict", err)
		}
	})
}

func TestMediaFileService_Get(t *testing.T) {
	repo := newFakeMediaFileRepository()
	svc := service.NewMediaFileService(repo)

	if _, err := svc.Create(context.Background(), validMediaFile("m1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "m1"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing media file returned %v, want ErrNotFound", err)
	}
}

func TestMediaFileService_Update(t *testing.T) {
	t.Run("valid update is persisted", func(t *testing.T) {
		repo := newFakeMediaFileRepository()
		svc := service.NewMediaFileService(repo)

		if _, err := svc.Create(context.Background(), validMediaFile("m1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		updated := validMediaFile("m1")
		updated.Quality = "1080p"
		if _, err := svc.Update(context.Background(), updated); err != nil {
			t.Fatalf("Update returned error: %v", err)
		}

		got, err := svc.Get(context.Background(), "m1")
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.Quality != "1080p" {
			t.Fatalf("Get after Update returned Quality %q, want %q", got.Quality, "1080p")
		}
	})

	t.Run("update of a missing media file returns ErrNotFound", func(t *testing.T) {
		repo := newFakeMediaFileRepository()
		svc := service.NewMediaFileService(repo)

		if _, err := svc.Update(context.Background(), validMediaFile("missing")); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Update on missing media file returned %v, want ErrNotFound", err)
		}
	})
}

func TestMediaFileService_Delete(t *testing.T) {
	repo := newFakeMediaFileRepository()
	svc := service.NewMediaFileService(repo)

	if _, err := svc.Create(context.Background(), validMediaFile("m1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(context.Background(), "m1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "m1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestMediaFileService_List(t *testing.T) {
	repo := newFakeMediaFileRepository()
	svc := service.NewMediaFileService(repo)

	for _, id := range []string{"m1", "m2"} {
		if _, err := svc.Create(context.Background(), validMediaFile(id)); err != nil {
			t.Fatalf("Create(%q) returned error: %v", id, err)
		}
	}

	files, _, err := svc.List(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("List returned %d media files, want 2", len(files))
	}
}
