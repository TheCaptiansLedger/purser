package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeEntryPersonRepository struct {
	byKey map[string]*domain.EntryPerson
}

func newFakeEntryPersonRepository() *fakeEntryPersonRepository {
	return &fakeEntryPersonRepository{byKey: make(map[string]*domain.EntryPerson)}
}

func entryPersonKey(libraryEntryID, personID, role string) string {
	return libraryEntryID + "|" + personID + "|" + role
}

func (f *fakeEntryPersonRepository) Create(_ context.Context, ep *domain.EntryPerson) error {
	k := entryPersonKey(ep.LibraryEntryID, ep.PersonID, ep.Role)
	if _, exists := f.byKey[k]; exists {
		return ports.ErrConflict
	}
	stored := *ep
	f.byKey[k] = &stored
	return nil
}

func (f *fakeEntryPersonRepository) Get(_ context.Context, libraryEntryID, personID, role string) (*domain.EntryPerson, error) {
	ep, ok := f.byKey[entryPersonKey(libraryEntryID, personID, role)]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *ep
	return &stored, nil
}

func (f *fakeEntryPersonRepository) Update(_ context.Context, ep *domain.EntryPerson) error {
	k := entryPersonKey(ep.LibraryEntryID, ep.PersonID, ep.Role)
	if _, ok := f.byKey[k]; !ok {
		return ports.ErrNotFound
	}
	stored := *ep
	f.byKey[k] = &stored
	return nil
}

func (f *fakeEntryPersonRepository) Delete(_ context.Context, libraryEntryID, personID, role string) error {
	k := entryPersonKey(libraryEntryID, personID, role)
	if _, ok := f.byKey[k]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byKey, k)
	return nil
}

func (f *fakeEntryPersonRepository) List(_ context.Context, _, _ string, _ int, _ string) ([]*domain.EntryPerson, string, error) {
	rows := make([]*domain.EntryPerson, 0, len(f.byKey))
	for _, ep := range f.byKey {
		stored := *ep
		rows = append(rows, &stored)
	}
	return rows, "", nil
}

func validEntryPerson(libraryEntryID, personID, role string) *domain.EntryPerson {
	return &domain.EntryPerson{LibraryEntryID: libraryEntryID, PersonID: personID, Role: role}
}

func TestEntryPersonService_Create(t *testing.T) {
	t.Run("valid credit is persisted", func(t *testing.T) {
		repo := newFakeEntryPersonRepository()
		svc := service.NewEntryPersonService(repo)

		got, err := svc.Create(context.Background(), validEntryPerson("e1", "p1", "director"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.Role != "director" {
			t.Fatalf("Create returned Role %q, want %q", got.Role, "director")
		}
	})

	t.Run("invalid credit is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeEntryPersonRepository()
		svc := service.NewEntryPersonService(repo)

		invalid := validEntryPerson("e1", "p1", "")
		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid credit returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeEntryPersonRepository()
		svc := service.NewEntryPersonService(repo)

		if _, err := svc.Create(context.Background(), validEntryPerson("e1", "p1", "director")); err != nil {
			t.Fatalf("first Create returned error: %v", err)
		}
		if _, err := svc.Create(context.Background(), validEntryPerson("e1", "p1", "director")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("duplicate Create returned %v, want ErrConflict", err)
		}
	})
}

func TestEntryPersonService_Get(t *testing.T) {
	repo := newFakeEntryPersonRepository()
	svc := service.NewEntryPersonService(repo)

	if _, err := svc.Create(context.Background(), validEntryPerson("e1", "p1", "director")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "e1", "p1", "director"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "e1", "missing", "director"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing credit returned %v, want ErrNotFound", err)
	}
}

func TestEntryPersonService_Update(t *testing.T) {
	repo := newFakeEntryPersonRepository()
	svc := service.NewEntryPersonService(repo)

	if _, err := svc.Create(context.Background(), validEntryPerson("e1", "p1", "director")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	updated := validEntryPerson("e1", "p1", "director")
	updated.CreditedAs = "New Credit"
	if _, err := svc.Update(context.Background(), updated); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := svc.Get(context.Background(), "e1", "p1", "director")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.CreditedAs != "New Credit" {
		t.Fatalf("Get after Update returned CreditedAs %q, want %q", got.CreditedAs, "New Credit")
	}

	if _, err := svc.Update(context.Background(), validEntryPerson("e1", "missing", "director")); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing credit returned %v, want ErrNotFound", err)
	}

	invalid := validEntryPerson("e1", "p1", "director")
	invalid.Role = ""
	var verr *domain.ValidationError
	if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
		t.Fatalf("Update with invalid credit returned %v, want *domain.ValidationError", err)
	}
}

func TestEntryPersonService_Delete(t *testing.T) {
	repo := newFakeEntryPersonRepository()
	svc := service.NewEntryPersonService(repo)

	if _, err := svc.Create(context.Background(), validEntryPerson("e1", "p1", "director")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(context.Background(), "e1", "p1", "director"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "e1", "p1", "director"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestEntryPersonService_List(t *testing.T) {
	repo := newFakeEntryPersonRepository()
	svc := service.NewEntryPersonService(repo)

	if _, err := svc.Create(context.Background(), validEntryPerson("e1", "p1", "director")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	rows, _, err := svc.List(context.Background(), "", "", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("List returned %d rows, want 1", len(rows))
	}
}
