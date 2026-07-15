package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	musicdomain "purser/internal/domain/music"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeMusicReleaseRepository struct {
	byID          map[string]*musicdomain.Release
	forceConflict bool
}

func newFakeMusicReleaseRepository() *fakeMusicReleaseRepository {
	return &fakeMusicReleaseRepository{byID: make(map[string]*musicdomain.Release)}
}

func (f *fakeMusicReleaseRepository) Create(_ context.Context, r *musicdomain.Release) error {
	if f.forceConflict {
		return ports.ErrConflict
	}
	if _, exists := f.byID[r.ID]; exists {
		return ports.ErrConflict
	}
	stored := *r
	f.byID[r.ID] = &stored
	return nil
}

func (f *fakeMusicReleaseRepository) Get(_ context.Context, id string) (*musicdomain.Release, error) {
	r, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *r
	return &stored, nil
}

func (f *fakeMusicReleaseRepository) Update(_ context.Context, r *musicdomain.Release) error {
	if _, exists := f.byID[r.ID]; !exists {
		return ports.ErrNotFound
	}
	stored := *r
	f.byID[r.ID] = &stored
	return nil
}

func (f *fakeMusicReleaseRepository) Delete(_ context.Context, id string) error {
	if _, exists := f.byID[id]; !exists {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeMusicReleaseRepository) List(_ context.Context, pageSize int, _ string) ([]*musicdomain.Release, string, error) {
	releases := make([]*musicdomain.Release, 0, len(f.byID))
	for _, r := range f.byID {
		stored := *r
		releases = append(releases, &stored)
	}
	if pageSize > 0 && len(releases) > pageSize {
		releases = releases[:pageSize]
	}
	return releases, "", nil
}

func validRelease(id string) *musicdomain.Release {
	return &musicdomain.Release{
		ID:             id,
		GroupID:        "group1",
		LibraryEntryID: "entry1",
		Title:          "Test Release",
		Status:         musicdomain.ReleaseStatusStub,
	}
}

func TestMusicReleaseService_Create(t *testing.T) {
	t.Run("valid release is persisted", func(t *testing.T) {
		repo := newFakeMusicReleaseRepository()
		svc := service.NewMusicReleaseService(repo)

		got, err := svc.Create(context.Background(), validRelease("r1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.ID == "" || got.ID == "r1" {
			t.Fatalf("Create returned ID %q, want a server-generated one", got.ID)
		}
	})

	t.Run("invalid release is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeMusicReleaseRepository()
		svc := service.NewMusicReleaseService(repo)

		invalid := validRelease("r1")
		invalid.Title = ""

		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid release returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeMusicReleaseRepository()
		repo.forceConflict = true
		svc := service.NewMusicReleaseService(repo)

		if _, err := svc.Create(context.Background(), validRelease("r1")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("Create returned %v, want ErrConflict", err)
		}
	})
}

func TestMusicReleaseService_Get(t *testing.T) {
	repo := newFakeMusicReleaseRepository()
	svc := service.NewMusicReleaseService(repo)

	created, err := svc.Create(context.Background(), validRelease("r1"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), created.ID); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing release returned %v, want ErrNotFound", err)
	}
}

func TestMusicReleaseService_Update(t *testing.T) {
	t.Run("valid update is persisted", func(t *testing.T) {
		repo := newFakeMusicReleaseRepository()
		svc := service.NewMusicReleaseService(repo)

		created, err := svc.Create(context.Background(), validRelease("r1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		updated := validRelease(created.ID)
		updated.Title = "Updated"
		if _, err := svc.Update(context.Background(), updated); err != nil {
			t.Fatalf("Update returned error: %v", err)
		}

		got, err := svc.Get(context.Background(), created.ID)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.Title != "Updated" {
			t.Fatalf("Get after Update returned Title %q, want %q", got.Title, "Updated")
		}
	})

	t.Run("invalid release is rejected", func(t *testing.T) {
		repo := newFakeMusicReleaseRepository()
		svc := service.NewMusicReleaseService(repo)

		if _, err := svc.Create(context.Background(), validRelease("r1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		invalid := validRelease("r1")
		invalid.Title = ""
		var verr *domain.ValidationError
		if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Update with invalid release returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("update of a missing release returns ErrNotFound", func(t *testing.T) {
		repo := newFakeMusicReleaseRepository()
		svc := service.NewMusicReleaseService(repo)

		if _, err := svc.Update(context.Background(), validRelease("missing")); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Update on missing release returned %v, want ErrNotFound", err)
		}
	})
}

func TestMusicReleaseService_Delete(t *testing.T) {
	repo := newFakeMusicReleaseRepository()
	svc := service.NewMusicReleaseService(repo)

	created, err := svc.Create(context.Background(), validRelease("r1"))
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

func TestMusicReleaseService_List(t *testing.T) {
	repo := newFakeMusicReleaseRepository()
	svc := service.NewMusicReleaseService(repo)

	for _, id := range []string{"r1", "r2"} {
		if _, err := svc.Create(context.Background(), validRelease(id)); err != nil {
			t.Fatalf("Create(%q) returned error: %v", id, err)
		}
	}

	releases, _, err := svc.List(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(releases) != 2 {
		t.Fatalf("List returned %d releases, want 2", len(releases))
	}
}
