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
