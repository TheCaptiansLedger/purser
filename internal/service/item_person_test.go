package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeItemPersonRepository struct {
	byKey map[string]*domain.ItemPerson
}

func newFakeItemPersonRepository() *fakeItemPersonRepository {
	return &fakeItemPersonRepository{byKey: make(map[string]*domain.ItemPerson)}
}

func itemPersonKey(itemID, personID, role string) string {
	return itemID + "|" + personID + "|" + role
}

func (f *fakeItemPersonRepository) Create(_ context.Context, ip *domain.ItemPerson) error {
	k := itemPersonKey(ip.ItemID, ip.PersonID, ip.Role)
	if _, exists := f.byKey[k]; exists {
		return ports.ErrConflict
	}
	stored := *ip
	f.byKey[k] = &stored
	return nil
}

func (f *fakeItemPersonRepository) Get(_ context.Context, itemID, personID, role string) (*domain.ItemPerson, error) {
	ip, ok := f.byKey[itemPersonKey(itemID, personID, role)]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *ip
	return &stored, nil
}

func (f *fakeItemPersonRepository) Update(_ context.Context, ip *domain.ItemPerson) error {
	k := itemPersonKey(ip.ItemID, ip.PersonID, ip.Role)
	if _, ok := f.byKey[k]; !ok {
		return ports.ErrNotFound
	}
	stored := *ip
	f.byKey[k] = &stored
	return nil
}

func (f *fakeItemPersonRepository) Delete(_ context.Context, itemID, personID, role string) error {
	k := itemPersonKey(itemID, personID, role)
	if _, ok := f.byKey[k]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byKey, k)
	return nil
}

func (f *fakeItemPersonRepository) List(_ context.Context, _, _ string, _ int, _ string) ([]*domain.ItemPerson, string, error) {
	rows := make([]*domain.ItemPerson, 0, len(f.byKey))
	for _, ip := range f.byKey {
		stored := *ip
		rows = append(rows, &stored)
	}
	return rows, "", nil
}

func validItemPerson(itemID, personID, role string) *domain.ItemPerson {
	return &domain.ItemPerson{ItemID: itemID, PersonID: personID, Role: role}
}

func TestItemPersonService_Create(t *testing.T) {
	t.Run("valid credit is persisted", func(t *testing.T) {
		repo := newFakeItemPersonRepository()
		svc := service.NewItemPersonService(repo)

		got, err := svc.Create(context.Background(), validItemPerson("i1", "p1", "performer"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.Role != "performer" {
			t.Fatalf("Create returned Role %q, want %q", got.Role, "performer")
		}
	})

	t.Run("invalid credit is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeItemPersonRepository()
		svc := service.NewItemPersonService(repo)

		invalid := validItemPerson("i1", "p1", "")
		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid credit returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeItemPersonRepository()
		svc := service.NewItemPersonService(repo)

		if _, err := svc.Create(context.Background(), validItemPerson("i1", "p1", "performer")); err != nil {
			t.Fatalf("first Create returned error: %v", err)
		}
		if _, err := svc.Create(context.Background(), validItemPerson("i1", "p1", "performer")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("duplicate Create returned %v, want ErrConflict", err)
		}
	})
}

func TestItemPersonService_Get(t *testing.T) {
	repo := newFakeItemPersonRepository()
	svc := service.NewItemPersonService(repo)

	if _, err := svc.Create(context.Background(), validItemPerson("i1", "p1", "performer")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "i1", "p1", "performer"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "i1", "missing", "performer"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing credit returned %v, want ErrNotFound", err)
	}
}

func TestItemPersonService_Update(t *testing.T) {
	repo := newFakeItemPersonRepository()
	svc := service.NewItemPersonService(repo)

	if _, err := svc.Create(context.Background(), validItemPerson("i1", "p1", "performer")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	updated := validItemPerson("i1", "p1", "performer")
	updated.CreditedAs = "New Credit"
	if _, err := svc.Update(context.Background(), updated); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := svc.Get(context.Background(), "i1", "p1", "performer")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.CreditedAs != "New Credit" {
		t.Fatalf("Get after Update returned CreditedAs %q, want %q", got.CreditedAs, "New Credit")
	}

	if _, err := svc.Update(context.Background(), validItemPerson("i1", "missing", "performer")); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing credit returned %v, want ErrNotFound", err)
	}

	invalid := validItemPerson("i1", "p1", "performer")
	invalid.Role = ""
	var verr *domain.ValidationError
	if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
		t.Fatalf("Update with invalid credit returned %v, want *domain.ValidationError", err)
	}
}

func TestItemPersonService_Delete(t *testing.T) {
	repo := newFakeItemPersonRepository()
	svc := service.NewItemPersonService(repo)

	if _, err := svc.Create(context.Background(), validItemPerson("i1", "p1", "performer")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(context.Background(), "i1", "p1", "performer"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "i1", "p1", "performer"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestItemPersonService_List(t *testing.T) {
	repo := newFakeItemPersonRepository()
	svc := service.NewItemPersonService(repo)

	if _, err := svc.Create(context.Background(), validItemPerson("i1", "p1", "performer")); err != nil {
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
