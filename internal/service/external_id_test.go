package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeExternalIDRepository struct {
	byKey map[string]*domain.ExternalID
}

func newFakeExternalIDRepository() *fakeExternalIDRepository {
	return &fakeExternalIDRepository{byKey: make(map[string]*domain.ExternalID)}
}

func externalIDKey(entityType domain.EntityType, entityID, source string) string {
	return string(entityType) + "|" + entityID + "|" + source
}

func (f *fakeExternalIDRepository) Create(_ context.Context, e *domain.ExternalID) error {
	k := externalIDKey(e.EntityType, e.EntityID, string(e.Source))
	if _, exists := f.byKey[k]; exists {
		return ports.ErrConflict
	}
	stored := *e
	f.byKey[k] = &stored
	return nil
}

func (f *fakeExternalIDRepository) Get(_ context.Context, entityType domain.EntityType, entityID, source string) (*domain.ExternalID, error) {
	e, ok := f.byKey[externalIDKey(entityType, entityID, source)]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *e
	return &stored, nil
}

func (f *fakeExternalIDRepository) GetByValue(_ context.Context, entityType domain.EntityType, source domain.ExternalIDSource, value string) (*domain.ExternalID, error) {
	for _, e := range f.byKey {
		if e.EntityType == entityType && e.Source == source && e.Value == value {
			stored := *e
			return &stored, nil
		}
	}
	return nil, ports.ErrNotFound
}

func (f *fakeExternalIDRepository) Update(_ context.Context, e *domain.ExternalID) error {
	k := externalIDKey(e.EntityType, e.EntityID, string(e.Source))
	if _, ok := f.byKey[k]; !ok {
		return ports.ErrNotFound
	}
	stored := *e
	f.byKey[k] = &stored
	return nil
}

func (f *fakeExternalIDRepository) Delete(_ context.Context, entityType domain.EntityType, entityID, source string) error {
	k := externalIDKey(entityType, entityID, source)
	if _, ok := f.byKey[k]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byKey, k)
	return nil
}

func (f *fakeExternalIDRepository) List(_ context.Context, _ domain.EntityType, _ string, _ int, _ string) ([]*domain.ExternalID, string, error) {
	rows := make([]*domain.ExternalID, 0, len(f.byKey))
	for _, e := range f.byKey {
		stored := *e
		rows = append(rows, &stored)
	}
	return rows, "", nil
}

func validExternalID(entityID, source string) *domain.ExternalID {
	return &domain.ExternalID{EntityType: domain.EntityTypePerson, EntityID: entityID, Source: domain.ExternalIDSource(source), Value: "v1"}
}

func TestExternalIDService_Create(t *testing.T) {
	t.Run("valid external id is persisted", func(t *testing.T) {
		repo := newFakeExternalIDRepository()
		svc := service.NewExternalIDService(repo)

		got, err := svc.Create(context.Background(), validExternalID("p1", "stashdb"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.EntityID != "p1" {
			t.Fatalf("Create returned EntityID %q, want %q", got.EntityID, "p1")
		}
	})

	t.Run("invalid external id is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeExternalIDRepository()
		svc := service.NewExternalIDService(repo)

		invalid := validExternalID("p1", "stashdb")
		invalid.Value = ""
		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid external id returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeExternalIDRepository()
		svc := service.NewExternalIDService(repo)

		if _, err := svc.Create(context.Background(), validExternalID("p1", "stashdb")); err != nil {
			t.Fatalf("first Create returned error: %v", err)
		}
		if _, err := svc.Create(context.Background(), validExternalID("p1", "stashdb")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("duplicate Create returned %v, want ErrConflict", err)
		}
	})
}

func TestExternalIDService_Get(t *testing.T) {
	repo := newFakeExternalIDRepository()
	svc := service.NewExternalIDService(repo)

	if _, err := svc.Create(context.Background(), validExternalID("p1", "stashdb")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), domain.EntityTypePerson, "p1", "stashdb"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), domain.EntityTypePerson, "missing", "stashdb"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing external id returned %v, want ErrNotFound", err)
	}
}

func TestExternalIDService_Update(t *testing.T) {
	repo := newFakeExternalIDRepository()
	svc := service.NewExternalIDService(repo)

	if _, err := svc.Create(context.Background(), validExternalID("p1", "stashdb")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	updated := validExternalID("p1", "stashdb")
	updated.Value = "new-value"
	if _, err := svc.Update(context.Background(), updated); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	got, err := svc.Get(context.Background(), domain.EntityTypePerson, "p1", "stashdb")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Value != "new-value" {
		t.Fatalf("Get after Update returned Value %q, want %q", got.Value, "new-value")
	}

	if _, err := svc.Update(context.Background(), validExternalID("missing", "stashdb")); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Update on missing external id returned %v, want ErrNotFound", err)
	}

	invalid := validExternalID("p1", "stashdb")
	invalid.Value = ""
	var verr *domain.ValidationError
	if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
		t.Fatalf("Update with invalid external id returned %v, want *domain.ValidationError", err)
	}
}

func TestExternalIDService_Delete(t *testing.T) {
	repo := newFakeExternalIDRepository()
	svc := service.NewExternalIDService(repo)

	if _, err := svc.Create(context.Background(), validExternalID("p1", "stashdb")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(context.Background(), domain.EntityTypePerson, "p1", "stashdb"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), domain.EntityTypePerson, "p1", "stashdb"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestExternalIDService_List(t *testing.T) {
	repo := newFakeExternalIDRepository()
	svc := service.NewExternalIDService(repo)

	if _, err := svc.Create(context.Background(), validExternalID("p1", "stashdb")); err != nil {
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
