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

type deletionFakePersonRepository struct {
	byID      map[string]*domain.Person
	deleteErr error
}

func (f *deletionFakePersonRepository) Create(_ context.Context, p *domain.Person) error {
	f.byID[p.ID] = p
	return nil
}

func (f *deletionFakePersonRepository) Get(_ context.Context, id string) (*domain.Person, error) {
	p, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return p, nil
}

func (f *deletionFakePersonRepository) Update(_ context.Context, p *domain.Person) error {
	f.byID[p.ID] = p
	return nil
}

func (f *deletionFakePersonRepository) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *deletionFakePersonRepository) List(context.Context, int, string) ([]*domain.Person, string, error) {
	return nil, "", nil
}

type deletionFakeEntryPersonRepository struct {
	rows      []*domain.EntryPerson
	listErr   error
	deleteErr error
}

func (f *deletionFakeEntryPersonRepository) Create(_ context.Context, ep *domain.EntryPerson) error {
	f.rows = append(f.rows, ep)
	return nil
}

func (f *deletionFakeEntryPersonRepository) Get(context.Context, string, string, string) (*domain.EntryPerson, error) {
	return nil, ports.ErrNotFound
}

func (f *deletionFakeEntryPersonRepository) Update(context.Context, *domain.EntryPerson) error {
	return nil
}

func (f *deletionFakeEntryPersonRepository) Delete(_ context.Context, libraryEntryID, personID, role string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	for i, ep := range f.rows {
		if ep.LibraryEntryID == libraryEntryID && ep.PersonID == personID && ep.Role == role {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return ports.ErrNotFound
}

func (f *deletionFakeEntryPersonRepository) List(_ context.Context, libraryEntryID, personID string, _ int, _ string) ([]*domain.EntryPerson, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var matched []*domain.EntryPerson
	for _, ep := range f.rows {
		if libraryEntryID != "" && ep.LibraryEntryID != libraryEntryID {
			continue
		}
		if personID != "" && ep.PersonID != personID {
			continue
		}
		matched = append(matched, ep)
	}
	return matched, "", nil
}

func newPersonDeletionFixture(withProfile bool) (
	*service.PersonDeletionService,
	*deletionFakePersonRepository,
	*deletionFakeEntryPersonRepository,
	*deletionFakeItemPersonRepository,
	*deletionFakeExternalIDRepository,
	*deletionFakeImageRepository,
	*deletionFakeTagAssignmentRepository,
	*browseFakePerformerProfileRepository,
) {
	people := &deletionFakePersonRepository{byID: map[string]*domain.Person{"p1": {ID: "p1"}}}
	entryPeople := &deletionFakeEntryPersonRepository{rows: []*domain.EntryPerson{
		{LibraryEntryID: "e1", PersonID: "p1", Role: "member"},
		{LibraryEntryID: "e2", PersonID: "p2", Role: "member"},
	}}
	itemPeople := &deletionFakeItemPersonRepository{rows: []*domain.ItemPerson{
		{ItemID: "i1", PersonID: "p1", Role: "performer"},
		{ItemID: "i2", PersonID: "p2", Role: "performer"},
	}}
	externalIDs := &deletionFakeExternalIDRepository{rows: []*domain.ExternalID{
		{EntityType: domain.EntityTypePerson, EntityID: "p1", Source: domain.ExternalIDSourceStashDB, Value: "1"},
	}}
	images := &deletionFakeImageRepository{byID: map[string]*domain.Image{
		"img1": {ID: "img1", OwnerType: "person", OwnerID: "p1"},
	}}
	tagAssignments := &deletionFakeTagAssignmentRepository{rows: []*domain.TagAssignment{
		{TagID: "t1", EntityType: domain.EntityTypePerson, EntityID: "p1"},
	}}
	profiles := &browseFakePerformerProfileRepository{byID: map[string]*afterdark.PerformerProfile{}}
	if withProfile {
		profiles.byID["p1"] = &afterdark.PerformerProfile{PersonID: "p1"}
	}

	svc := service.NewPersonDeletionService(people, entryPeople, itemPeople, externalIDs, images, tagAssignments, profiles)
	return svc, people, entryPeople, itemPeople, externalIDs, images, tagAssignments, profiles
}

func TestPersonDeletionService_GetDeletionImpact(t *testing.T) {
	svc, _, _, _, _, _, _, _ := newPersonDeletionFixture(true)

	impact, err := svc.GetDeletionImpact(context.Background(), "p1")
	if err != nil {
		t.Fatalf("GetDeletionImpact returned error: %v", err)
	}
	want := map[string]int{"entry_person": 1, "item_person": 1, "external_id": 1, "image": 1, "tag_assignment": 1, "performer_profile": 1}
	for _, row := range impact.Impacts {
		if row.Blocking {
			t.Fatalf("Impacts row %q marked Blocking, want false — Person never blocks a delete", row.Kind)
		}
		if want[row.Kind] != row.Count {
			t.Fatalf("Impacts row %q Count = %d, want %d", row.Kind, row.Count, want[row.Kind])
		}
	}

	if _, err := svc.GetDeletionImpact(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetDeletionImpact on missing person returned %v, want ErrNotFound", err)
	}
}

func TestPersonDeletionService_GetDeletionImpact_NoProfile(t *testing.T) {
	svc, _, _, _, _, _, _, _ := newPersonDeletionFixture(false)

	impact, err := svc.GetDeletionImpact(context.Background(), "p1")
	if err != nil {
		t.Fatalf("GetDeletionImpact returned error: %v", err)
	}
	for _, row := range impact.Impacts {
		if row.Kind == "performer_profile" && row.Count != 0 {
			t.Fatalf("performer_profile Count = %d, want 0 for a person with no profile", row.Count)
		}
	}
}

func TestPersonDeletionService_Delete(t *testing.T) {
	svc, people, entryPeople, itemPeople, externalIDs, images, tagAssignments, profiles := newPersonDeletionFixture(true)

	if err := svc.Delete(context.Background(), "p1", false); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if _, err := people.Get(context.Background(), "p1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("Delete did not remove the Person itself")
	}

	remainingEntryPeople, _, _ := entryPeople.List(context.Background(), "", "", 10, "")
	if len(remainingEntryPeople) != 1 || remainingEntryPeople[0].PersonID != "p2" {
		t.Fatalf("Delete left %+v, want only the p2 credit untouched", remainingEntryPeople)
	}

	remainingItemPeople, _, _ := itemPeople.List(context.Background(), "", "", 10, "")
	if len(remainingItemPeople) != 1 || remainingItemPeople[0].PersonID != "p2" {
		t.Fatalf("Delete left %+v, want only the p2 credit untouched", remainingItemPeople)
	}

	remainingExternalIDs, _, _ := externalIDs.List(context.Background(), "", "", 10, "")
	if len(remainingExternalIDs) != 0 {
		t.Fatalf("Delete left %+v external IDs, want none", remainingExternalIDs)
	}

	remainingImages, _, _ := images.List(context.Background(), "", "", 10, "")
	if len(remainingImages) != 0 {
		t.Fatalf("Delete left %+v images, want none", remainingImages)
	}

	remainingTagAssignments, _, _ := tagAssignments.List(context.Background(), "", "", "", 10, "")
	if len(remainingTagAssignments) != 0 {
		t.Fatalf("Delete left %+v tag assignments, want none", remainingTagAssignments)
	}

	if _, err := profiles.Get(context.Background(), "p1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("Delete did not remove the PerformerProfile")
	}
}

func TestPersonDeletionService_Delete_NoProfile(t *testing.T) {
	svc, people, _, _, _, _, _, _ := newPersonDeletionFixture(false)

	if err := svc.Delete(context.Background(), "p1", false); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := people.Get(context.Background(), "p1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("Delete did not remove the Person itself")
	}
}

func TestPersonDeletionService_DeleteMissing(t *testing.T) {
	svc, _, _, _, _, _, _, _ := newPersonDeletionFixture(true)

	if err := svc.Delete(context.Background(), "missing", false); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing person returned %v, want ErrNotFound", err)
	}
}

func TestPersonDeletionService_Delete_PropagatesPortErrors(t *testing.T) {
	t.Run("entryPeople List error propagates", func(t *testing.T) {
		svc, _, entryPeople, _, _, _, _, _ := newPersonDeletionFixture(true)
		entryPeople.listErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("entryPeople Delete error propagates", func(t *testing.T) {
		svc, _, entryPeople, _, _, _, _, _ := newPersonDeletionFixture(true)
		entryPeople.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("itemPeople List error propagates", func(t *testing.T) {
		svc, _, _, itemPeople, _, _, _, _ := newPersonDeletionFixture(true)
		itemPeople.listErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("itemPeople Delete error propagates", func(t *testing.T) {
		svc, _, _, itemPeople, _, _, _, _ := newPersonDeletionFixture(true)
		itemPeople.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("externalIDs List error propagates", func(t *testing.T) {
		svc, _, _, _, externalIDs, _, _, _ := newPersonDeletionFixture(true)
		externalIDs.listErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("externalIDs Delete error propagates", func(t *testing.T) {
		svc, _, _, _, externalIDs, _, _, _ := newPersonDeletionFixture(true)
		externalIDs.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("images List error propagates", func(t *testing.T) {
		svc, _, _, _, _, images, _, _ := newPersonDeletionFixture(true)
		images.listErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("images Delete error propagates", func(t *testing.T) {
		svc, _, _, _, _, images, _, _ := newPersonDeletionFixture(true)
		images.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("tagAssignments List error propagates", func(t *testing.T) {
		svc, _, _, _, _, _, tagAssignments, _ := newPersonDeletionFixture(true)
		tagAssignments.listErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("tagAssignments Delete error propagates", func(t *testing.T) {
		svc, _, _, _, _, _, tagAssignments, _ := newPersonDeletionFixture(true)
		tagAssignments.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("profiles Get error (not ErrNotFound) propagates from GetDeletionImpact", func(t *testing.T) {
		svc, _, _, _, _, _, _, profiles := newPersonDeletionFixture(false)
		profiles.getErr = errBoom
		if _, err := svc.GetDeletionImpact(context.Background(), "p1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})

	t.Run("profiles Get error (not ErrNotFound) propagates from Delete", func(t *testing.T) {
		svc, _, _, _, _, _, _, profiles := newPersonDeletionFixture(false)
		profiles.getErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("profiles Delete error propagates", func(t *testing.T) {
		svc, _, _, _, _, _, _, profiles := newPersonDeletionFixture(true)
		profiles.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("people Delete error propagates", func(t *testing.T) {
		svc, people, _, _, _, _, _, _ := newPersonDeletionFixture(true)
		people.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "p1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})
}
