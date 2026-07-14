package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
	"time"
)

// fakePersonRepository is a minimal in-memory ports.PersonRepository test
// double, scoped to this test file only — per ADR 0003, service tests run
// against a fake port, never a real adapter.
type fakePersonRepository struct {
	byID map[string]*domain.Person
	// forceConflict makes the next Create return ports.ErrConflict without
	// touching byID — with server-generated IDs (see
	// docs/adr/0020-server-generated-kernel-entity-ids.md), two Creates
	// can no longer be forced to collide by reusing a literal ID.
	forceConflict bool
}

func newFakePersonRepository() *fakePersonRepository {
	return &fakePersonRepository{byID: make(map[string]*domain.Person)}
}

func (f *fakePersonRepository) Create(_ context.Context, p *domain.Person) error {
	if f.forceConflict {
		return ports.ErrConflict
	}
	if _, exists := f.byID[p.ID]; exists {
		return ports.ErrConflict
	}
	stored := *p
	f.byID[p.ID] = &stored
	return nil
}

func (f *fakePersonRepository) Get(_ context.Context, id string) (*domain.Person, error) {
	p, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *p
	return &stored, nil
}

func (f *fakePersonRepository) Update(_ context.Context, p *domain.Person) error {
	if _, ok := f.byID[p.ID]; !ok {
		return ports.ErrNotFound
	}
	stored := *p
	f.byID[p.ID] = &stored
	return nil
}

func (f *fakePersonRepository) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakePersonRepository) List(_ context.Context, _ int, _ string) ([]*domain.Person, string, error) {
	people := make([]*domain.Person, 0, len(f.byID))
	for _, p := range f.byID {
		stored := *p
		people = append(people, &stored)
	}
	return people, "", nil
}

func validPerson(id string) *domain.Person {
	return &domain.Person{
		ID:          id,
		Name:        "Test Person",
		Gender:      domain.GenderUnknown,
		MonitorMode: domain.MonitorModeNone,
		AddedAt:     time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func TestPersonService_Create(t *testing.T) {
	t.Run("valid person is persisted", func(t *testing.T) {
		repo := newFakePersonRepository()
		svc := service.NewPersonService(repo)

		got, err := svc.Create(context.Background(), validPerson("p1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.ID == "" {
			t.Fatal("Create returned an empty ID, want a server-generated one")
		}
		if got.ID == "p1" {
			t.Fatal("Create returned the caller-supplied ID, want a server-generated one")
		}

		if _, err := repo.Get(context.Background(), got.ID); err != nil {
			t.Fatalf("repo.Get after Create returned error: %v", err)
		}
	})

	t.Run("invalid person is rejected before touching the repository", func(t *testing.T) {
		repo := newFakePersonRepository()
		svc := service.NewPersonService(repo)

		invalid := validPerson("p1")
		invalid.Name = ""

		var verr *domain.ValidationError
		_, err := svc.Create(context.Background(), invalid)
		if !errors.As(err, &verr) {
			t.Fatalf("Create with invalid person returned %v, want *domain.ValidationError", err)
		}
		if _, getErr := repo.Get(context.Background(), "p1"); !errors.Is(getErr, ports.ErrNotFound) {
			t.Fatal("Create with invalid person reached the repository")
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakePersonRepository()
		repo.forceConflict = true
		svc := service.NewPersonService(repo)

		if _, err := svc.Create(context.Background(), validPerson("p1")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("Create returned %v, want ErrConflict", err)
		}
	})
}

func TestPersonService_Get(t *testing.T) {
	repo := newFakePersonRepository()
	svc := service.NewPersonService(repo)

	created, err := svc.Create(context.Background(), validPerson("p1"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("Get returned ID %q, want %q", got.ID, created.ID)
	}

	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing person returned %v, want ErrNotFound", err)
	}
}

func TestPersonService_Update(t *testing.T) {
	t.Run("valid update is persisted", func(t *testing.T) {
		repo := newFakePersonRepository()
		svc := service.NewPersonService(repo)

		created, err := svc.Create(context.Background(), validPerson("p1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		updated := validPerson(created.ID)
		updated.Name = "Updated Name"
		if _, err := svc.Update(context.Background(), updated); err != nil {
			t.Fatalf("Update returned error: %v", err)
		}

		got, err := svc.Get(context.Background(), created.ID)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.Name != "Updated Name" {
			t.Fatalf("Get after Update returned Name %q, want %q", got.Name, "Updated Name")
		}
	})

	t.Run("invalid person is rejected before touching the repository", func(t *testing.T) {
		repo := newFakePersonRepository()
		svc := service.NewPersonService(repo)

		if _, err := svc.Create(context.Background(), validPerson("p1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		invalid := validPerson("p1")
		invalid.Name = ""

		var verr *domain.ValidationError
		if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Update with invalid person returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("update of a missing person returns ErrNotFound", func(t *testing.T) {
		repo := newFakePersonRepository()
		svc := service.NewPersonService(repo)

		if _, err := svc.Update(context.Background(), validPerson("missing")); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Update on missing person returned %v, want ErrNotFound", err)
		}
	})
}

func TestPersonService_Delete(t *testing.T) {
	repo := newFakePersonRepository()
	svc := service.NewPersonService(repo)

	created, err := svc.Create(context.Background(), validPerson("p1"))
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

func TestPersonService_List(t *testing.T) {
	repo := newFakePersonRepository()
	svc := service.NewPersonService(repo)

	for _, id := range []string{"p1", "p2"} {
		if _, err := svc.Create(context.Background(), validPerson(id)); err != nil {
			t.Fatalf("Create(%q) returned error: %v", id, err)
		}
	}

	people, _, err := svc.List(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(people) != 2 {
		t.Fatalf("List returned %d people, want 2", len(people))
	}
}
