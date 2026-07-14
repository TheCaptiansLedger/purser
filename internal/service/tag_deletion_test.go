package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// errBoom is a shared sentinel used across every *_deletion_test.go file in
// this package to prove a port's error propagates through the deletion
// service, rather than being swallowed or replaced.
var errBoom = errors.New("boom")

type deletionFakeTagRepository struct {
	byID      map[string]*domain.Tag
	deleteErr error
}

func newDeletionFakeTagRepository() *deletionFakeTagRepository {
	return &deletionFakeTagRepository{byID: make(map[string]*domain.Tag)}
}

func (f *deletionFakeTagRepository) Create(_ context.Context, t *domain.Tag) error {
	f.byID[t.ID] = t
	return nil
}

func (f *deletionFakeTagRepository) Get(_ context.Context, id string) (*domain.Tag, error) {
	t, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return t, nil
}

func (f *deletionFakeTagRepository) Update(_ context.Context, t *domain.Tag) error {
	f.byID[t.ID] = t
	return nil
}

func (f *deletionFakeTagRepository) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *deletionFakeTagRepository) List(context.Context, int, string) ([]*domain.Tag, string, error) {
	return nil, "", nil
}

type deletionFakeTagAssignmentRepository struct {
	rows      []*domain.TagAssignment
	listErr   error
	deleteErr error
}

func (f *deletionFakeTagAssignmentRepository) Create(_ context.Context, ta *domain.TagAssignment) error {
	f.rows = append(f.rows, ta)
	return nil
}

func (f *deletionFakeTagAssignmentRepository) Get(_ context.Context, tagID string, entityType domain.EntityType, entityID string) (*domain.TagAssignment, error) {
	for _, ta := range f.rows {
		if ta.TagID == tagID && ta.EntityType == entityType && ta.EntityID == entityID {
			return ta, nil
		}
	}
	return nil, ports.ErrNotFound
}

func (f *deletionFakeTagAssignmentRepository) Delete(_ context.Context, tagID string, entityType domain.EntityType, entityID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	for i, ta := range f.rows {
		if ta.TagID == tagID && ta.EntityType == entityType && ta.EntityID == entityID {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return ports.ErrNotFound
}

func (f *deletionFakeTagAssignmentRepository) List(_ context.Context, tagID string, entityType domain.EntityType, entityID string, _ int, _ string) ([]*domain.TagAssignment, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var matched []*domain.TagAssignment
	for _, ta := range f.rows {
		if tagID != "" && ta.TagID != tagID {
			continue
		}
		if entityType != "" && ta.EntityType != entityType {
			continue
		}
		if entityID != "" && ta.EntityID != entityID {
			continue
		}
		matched = append(matched, ta)
	}
	return matched, "", nil
}

func TestTagDeletionService_GetDeletionImpact(t *testing.T) {
	tags := newDeletionFakeTagRepository()
	tags.byID["t1"] = &domain.Tag{ID: "t1"}
	assignments := &deletionFakeTagAssignmentRepository{rows: []*domain.TagAssignment{
		{TagID: "t1", EntityType: domain.EntityTypePerson, EntityID: "p1"},
		{TagID: "t1", EntityType: domain.EntityTypeItem, EntityID: "i1"},
		{TagID: "t2", EntityType: domain.EntityTypePerson, EntityID: "p2"},
	}}
	svc := service.NewTagDeletionService(tags, assignments)

	impact, err := svc.GetDeletionImpact(context.Background(), "t1")
	if err != nil {
		t.Fatalf("GetDeletionImpact returned error: %v", err)
	}
	if len(impact.Impacts) != 1 || impact.Impacts[0].Count != 2 {
		t.Fatalf("GetDeletionImpact returned %+v, want a single row with Count 2", impact.Impacts)
	}
	if impact.Impacts[0].Blocking {
		t.Fatal("GetDeletionImpact marked tag_assignment as Blocking, want false — Tag never blocks a delete")
	}

	if _, err := svc.GetDeletionImpact(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetDeletionImpact on missing tag returned %v, want ErrNotFound", err)
	}
}

func TestTagDeletionService_Delete(t *testing.T) {
	tags := newDeletionFakeTagRepository()
	tags.byID["t1"] = &domain.Tag{ID: "t1"}
	assignments := &deletionFakeTagAssignmentRepository{rows: []*domain.TagAssignment{
		{TagID: "t1", EntityType: domain.EntityTypePerson, EntityID: "p1"},
		{TagID: "t1", EntityType: domain.EntityTypeItem, EntityID: "i1"},
		{TagID: "t2", EntityType: domain.EntityTypePerson, EntityID: "p2"},
	}}
	svc := service.NewTagDeletionService(tags, assignments)

	if err := svc.Delete(context.Background(), "t1", false); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if _, err := tags.Get(context.Background(), "t1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("Delete did not remove the Tag itself")
	}
	remaining, _, err := assignments.List(context.Background(), "", "", "", 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(remaining) != 1 || remaining[0].TagID != "t2" {
		t.Fatalf("Delete left %+v, want only the t2 assignment untouched", remaining)
	}
}

func TestTagDeletionService_DeleteMissing(t *testing.T) {
	tags := newDeletionFakeTagRepository()
	assignments := &deletionFakeTagAssignmentRepository{}
	svc := service.NewTagDeletionService(tags, assignments)

	if err := svc.Delete(context.Background(), "missing", false); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing tag returned %v, want ErrNotFound", err)
	}
}

func TestTagDeletionService_Delete_PropagatesPortErrors(t *testing.T) {
	t.Run("assignments List error propagates from GetDeletionImpact", func(t *testing.T) {
		tags := newDeletionFakeTagRepository()
		tags.byID["t1"] = &domain.Tag{ID: "t1"}
		assignments := &deletionFakeTagAssignmentRepository{listErr: errBoom}
		svc := service.NewTagDeletionService(tags, assignments)

		if _, err := svc.GetDeletionImpact(context.Background(), "t1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})

	t.Run("assignments List error propagates from Delete", func(t *testing.T) {
		tags := newDeletionFakeTagRepository()
		tags.byID["t1"] = &domain.Tag{ID: "t1"}
		assignments := &deletionFakeTagAssignmentRepository{listErr: errBoom}
		svc := service.NewTagDeletionService(tags, assignments)

		if err := svc.Delete(context.Background(), "t1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("assignments Delete error propagates", func(t *testing.T) {
		tags := newDeletionFakeTagRepository()
		tags.byID["t1"] = &domain.Tag{ID: "t1"}
		assignments := &deletionFakeTagAssignmentRepository{
			rows:      []*domain.TagAssignment{{TagID: "t1", EntityType: domain.EntityTypePerson, EntityID: "p1"}},
			deleteErr: errBoom,
		}
		svc := service.NewTagDeletionService(tags, assignments)

		if err := svc.Delete(context.Background(), "t1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("tags Delete error propagates", func(t *testing.T) {
		tags := newDeletionFakeTagRepository()
		tags.byID["t1"] = &domain.Tag{ID: "t1"}
		tags.deleteErr = errBoom
		assignments := &deletionFakeTagAssignmentRepository{}
		svc := service.NewTagDeletionService(tags, assignments)

		if err := svc.Delete(context.Background(), "t1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})
}
