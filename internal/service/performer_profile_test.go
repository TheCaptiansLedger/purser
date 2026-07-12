package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/domain/afterdark"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakePerformerProfileRepository struct {
	byID map[string]*afterdark.PerformerProfile
}

func newFakePerformerProfileRepository() *fakePerformerProfileRepository {
	return &fakePerformerProfileRepository{byID: make(map[string]*afterdark.PerformerProfile)}
}

func (f *fakePerformerProfileRepository) Create(_ context.Context, p *afterdark.PerformerProfile) error {
	if _, exists := f.byID[p.PersonID]; exists {
		return ports.ErrConflict
	}
	stored := *p
	f.byID[p.PersonID] = &stored
	return nil
}

func (f *fakePerformerProfileRepository) Get(_ context.Context, personID string) (*afterdark.PerformerProfile, error) {
	p, ok := f.byID[personID]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *p
	return &stored, nil
}

func (f *fakePerformerProfileRepository) Update(_ context.Context, p *afterdark.PerformerProfile) error {
	if _, ok := f.byID[p.PersonID]; !ok {
		return ports.ErrNotFound
	}
	stored := *p
	f.byID[p.PersonID] = &stored
	return nil
}

func (f *fakePerformerProfileRepository) Delete(_ context.Context, personID string) error {
	if _, ok := f.byID[personID]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, personID)
	return nil
}

func (f *fakePerformerProfileRepository) List(_ context.Context, _ int, _ string) ([]*afterdark.PerformerProfile, string, error) {
	profiles := make([]*afterdark.PerformerProfile, 0, len(f.byID))
	for _, p := range f.byID {
		stored := *p
		profiles = append(profiles, &stored)
	}
	return profiles, "", nil
}

func validPerformerProfile(personID string) *afterdark.PerformerProfile {
	return &afterdark.PerformerProfile{PersonID: personID, CupSize: "34"}
}

func TestPerformerProfileService_Create(t *testing.T) {
	t.Run("valid profile is persisted", func(t *testing.T) {
		repo := newFakePerformerProfileRepository()
		svc := service.NewPerformerProfileService(repo)

		got, err := svc.Create(context.Background(), validPerformerProfile("p1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.PersonID != "p1" {
			t.Fatalf("Create returned PersonID %q, want %q", got.PersonID, "p1")
		}
	})

	t.Run("invalid profile is rejected before touching the repository", func(t *testing.T) {
		repo := newFakePerformerProfileRepository()
		svc := service.NewPerformerProfileService(repo)

		invalid := validPerformerProfile("")
		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid profile returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakePerformerProfileRepository()
		svc := service.NewPerformerProfileService(repo)

		if _, err := svc.Create(context.Background(), validPerformerProfile("p1")); err != nil {
			t.Fatalf("first Create returned error: %v", err)
		}
		if _, err := svc.Create(context.Background(), validPerformerProfile("p1")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("duplicate Create returned %v, want ErrConflict", err)
		}
	})
}

func TestPerformerProfileService_Get(t *testing.T) {
	repo := newFakePerformerProfileRepository()
	svc := service.NewPerformerProfileService(repo)

	if _, err := svc.Create(context.Background(), validPerformerProfile("p1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "p1"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing profile returned %v, want ErrNotFound", err)
	}
}

func TestPerformerProfileService_Update(t *testing.T) {
	t.Run("valid update is persisted", func(t *testing.T) {
		repo := newFakePerformerProfileRepository()
		svc := service.NewPerformerProfileService(repo)

		if _, err := svc.Create(context.Background(), validPerformerProfile("p1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		updated := validPerformerProfile("p1")
		updated.CupSize = "36"
		if _, err := svc.Update(context.Background(), updated); err != nil {
			t.Fatalf("Update returned error: %v", err)
		}

		got, err := svc.Get(context.Background(), "p1")
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.CupSize != "36" {
			t.Fatalf("Get after Update returned CupSize %q, want %q", got.CupSize, "36")
		}
	})

	t.Run("invalid profile is rejected", func(t *testing.T) {
		repo := newFakePerformerProfileRepository()
		svc := service.NewPerformerProfileService(repo)

		if _, err := svc.Create(context.Background(), validPerformerProfile("p1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		invalid := validPerformerProfile("p1")
		invalid.PersonID = ""
		var verr *domain.ValidationError
		if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Update with invalid profile returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("update of a missing profile returns ErrNotFound", func(t *testing.T) {
		repo := newFakePerformerProfileRepository()
		svc := service.NewPerformerProfileService(repo)

		if _, err := svc.Update(context.Background(), validPerformerProfile("missing")); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Update on missing profile returned %v, want ErrNotFound", err)
		}
	})
}

func TestPerformerProfileService_Delete(t *testing.T) {
	repo := newFakePerformerProfileRepository()
	svc := service.NewPerformerProfileService(repo)

	if _, err := svc.Create(context.Background(), validPerformerProfile("p1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(context.Background(), "p1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "p1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestPerformerProfileService_List(t *testing.T) {
	repo := newFakePerformerProfileRepository()
	svc := service.NewPerformerProfileService(repo)

	for _, id := range []string{"p1", "p2"} {
		if _, err := svc.Create(context.Background(), validPerformerProfile(id)); err != nil {
			t.Fatalf("Create(%q) returned error: %v", id, err)
		}
	}

	profiles, _, err := svc.List(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("List returned %d profiles, want 2", len(profiles))
	}
}
