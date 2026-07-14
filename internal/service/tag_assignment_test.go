package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeTagAssignmentRepository struct {
	byKey          map[string]*domain.TagAssignment
	createBatchErr error
}

func newFakeTagAssignmentRepository() *fakeTagAssignmentRepository {
	return &fakeTagAssignmentRepository{byKey: make(map[string]*domain.TagAssignment)}
}

func tagAssignmentKey(tagID string, entityType domain.EntityType, entityID string) string {
	return tagID + "|" + string(entityType) + "|" + entityID
}

func (f *fakeTagAssignmentRepository) Create(_ context.Context, ta *domain.TagAssignment) error {
	k := tagAssignmentKey(ta.TagID, ta.EntityType, ta.EntityID)
	if _, exists := f.byKey[k]; exists {
		return ports.ErrConflict
	}
	stored := *ta
	f.byKey[k] = &stored
	return nil
}

func (f *fakeTagAssignmentRepository) Get(_ context.Context, tagID string, entityType domain.EntityType, entityID string) (*domain.TagAssignment, error) {
	ta, ok := f.byKey[tagAssignmentKey(tagID, entityType, entityID)]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *ta
	return &stored, nil
}

func (f *fakeTagAssignmentRepository) Delete(_ context.Context, tagID string, entityType domain.EntityType, entityID string) error {
	k := tagAssignmentKey(tagID, entityType, entityID)
	if _, ok := f.byKey[k]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byKey, k)
	return nil
}

func (f *fakeTagAssignmentRepository) List(_ context.Context, _ string, _ domain.EntityType, _ string, _ int, _ string) ([]*domain.TagAssignment, string, error) {
	rows := make([]*domain.TagAssignment, 0, len(f.byKey))
	for _, ta := range f.byKey {
		stored := *ta
		rows = append(rows, &stored)
	}
	return rows, "", nil
}

func (f *fakeTagAssignmentRepository) CreateBatch(_ context.Context, tas []*domain.TagAssignment) error {
	if f.createBatchErr != nil {
		return f.createBatchErr
	}
	for _, ta := range tas {
		k := tagAssignmentKey(ta.TagID, ta.EntityType, ta.EntityID)
		if _, exists := f.byKey[k]; exists {
			return ports.ErrConflict
		}
	}
	for _, ta := range tas {
		stored := *ta
		f.byKey[tagAssignmentKey(ta.TagID, ta.EntityType, ta.EntityID)] = &stored
	}
	return nil
}

func validTagAssignment(tagID string, entityType domain.EntityType, entityID string) *domain.TagAssignment {
	return &domain.TagAssignment{TagID: tagID, EntityType: entityType, EntityID: entityID}
}

func TestTagAssignmentService_Create(t *testing.T) {
	t.Run("valid assignment is persisted", func(t *testing.T) {
		repo := newFakeTagAssignmentRepository()
		svc := service.NewTagAssignmentService(repo)

		got, err := svc.Create(context.Background(), validTagAssignment("t1", domain.EntityTypePerson, "p1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.TagID != "t1" {
			t.Fatalf("Create returned TagID %q, want %q", got.TagID, "t1")
		}
	})

	t.Run("invalid assignment is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeTagAssignmentRepository()
		svc := service.NewTagAssignmentService(repo)

		invalid := validTagAssignment("", domain.EntityTypePerson, "p1")
		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid assignment returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeTagAssignmentRepository()
		svc := service.NewTagAssignmentService(repo)

		if _, err := svc.Create(context.Background(), validTagAssignment("t1", domain.EntityTypePerson, "p1")); err != nil {
			t.Fatalf("first Create returned error: %v", err)
		}
		if _, err := svc.Create(context.Background(), validTagAssignment("t1", domain.EntityTypePerson, "p1")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("duplicate Create returned %v, want ErrConflict", err)
		}
	})
}

func TestTagAssignmentService_Get(t *testing.T) {
	repo := newFakeTagAssignmentRepository()
	svc := service.NewTagAssignmentService(repo)

	if _, err := svc.Create(context.Background(), validTagAssignment("t1", domain.EntityTypePerson, "p1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "t1", domain.EntityTypePerson, "p1"); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "t1", domain.EntityTypePerson, "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing assignment returned %v, want ErrNotFound", err)
	}
}

func TestTagAssignmentService_Delete(t *testing.T) {
	repo := newFakeTagAssignmentRepository()
	svc := service.NewTagAssignmentService(repo)

	if _, err := svc.Create(context.Background(), validTagAssignment("t1", domain.EntityTypePerson, "p1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(context.Background(), "t1", domain.EntityTypePerson, "p1"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "t1", domain.EntityTypePerson, "p1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestTagAssignmentService_List(t *testing.T) {
	repo := newFakeTagAssignmentRepository()
	svc := service.NewTagAssignmentService(repo)

	if _, err := svc.Create(context.Background(), validTagAssignment("t1", domain.EntityTypePerson, "p1")); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	rows, _, err := svc.List(context.Background(), "", "", "", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("List returned %d rows, want 1", len(rows))
	}
}

func TestTagAssignmentService_BulkCreateTagAssignments(t *testing.T) {
	t.Run("valid entity ids are all persisted", func(t *testing.T) {
		repo := newFakeTagAssignmentRepository()
		svc := service.NewTagAssignmentService(repo)

		got, err := svc.BulkCreateTagAssignments(context.Background(), "t1", domain.EntityTypeItem, []string{"scene1", "scene2", "scene3"})
		if err != nil {
			t.Fatalf("BulkCreateTagAssignments returned error: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("BulkCreateTagAssignments returned %d assignments, want 3", len(got))
		}
		for _, entityID := range []string{"scene1", "scene2", "scene3"} {
			if _, err := svc.Get(context.Background(), "t1", domain.EntityTypeItem, entityID); err != nil {
				t.Fatalf("Get after BulkCreateTagAssignments for %q returned error: %v", entityID, err)
			}
		}
	})

	t.Run("an invalid row is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeTagAssignmentRepository()
		svc := service.NewTagAssignmentService(repo)

		var verr *domain.ValidationError
		_, err := svc.BulkCreateTagAssignments(context.Background(), "t1", domain.EntityTypeItem, []string{"scene1", ""})
		if !errors.As(err, &verr) {
			t.Fatalf("BulkCreateTagAssignments with an invalid entity id returned %v, want *domain.ValidationError", err)
		}
		if _, err := svc.Get(context.Background(), "t1", domain.EntityTypeItem, "scene1"); !errors.Is(err, ports.ErrNotFound) {
			t.Fatal("BulkCreateTagAssignments persisted scene1 despite a later row failing validation")
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeTagAssignmentRepository()
		repo.createBatchErr = ports.ErrConflict
		svc := service.NewTagAssignmentService(repo)

		_, err := svc.BulkCreateTagAssignments(context.Background(), "t1", domain.EntityTypeItem, []string{"scene1"})
		if !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("BulkCreateTagAssignments returned %v, want ErrConflict", err)
		}
	})
}
