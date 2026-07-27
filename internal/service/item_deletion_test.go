package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type deletionFakeItemRepository struct {
	byID           map[string]*domain.Item
	deleteErr      error
	deleteBatchErr error
}

func (f *deletionFakeItemRepository) Create(_ context.Context, i *domain.Item) error {
	f.byID[i.ID] = i
	return nil
}

func (f *deletionFakeItemRepository) Get(_ context.Context, id string) (*domain.Item, error) {
	i, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return i, nil
}

func (f *deletionFakeItemRepository) Update(_ context.Context, i *domain.Item) error {
	f.byID[i.ID] = i
	return nil
}

func (f *deletionFakeItemRepository) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *deletionFakeItemRepository) List(context.Context, string, string, string, int, string) ([]*domain.Item, string, error) {
	return nil, "", nil
}

func (f *deletionFakeItemRepository) DeleteBatch(_ context.Context, ids []string) error {
	if f.deleteBatchErr != nil {
		return f.deleteBatchErr
	}
	for _, id := range ids {
		if _, ok := f.byID[id]; !ok {
			return ports.ErrNotFound
		}
	}
	for _, id := range ids {
		delete(f.byID, id)
	}
	return nil
}

type deletionFakeItemPersonRepository struct {
	rows      []*domain.ItemPerson
	listErr   error
	deleteErr error
}

func (f *deletionFakeItemPersonRepository) Create(_ context.Context, ip *domain.ItemPerson) error {
	f.rows = append(f.rows, ip)
	return nil
}

func (f *deletionFakeItemPersonRepository) Get(context.Context, string, string, string) (*domain.ItemPerson, error) {
	return nil, ports.ErrNotFound
}

func (f *deletionFakeItemPersonRepository) Update(context.Context, *domain.ItemPerson) error {
	return nil
}

func (f *deletionFakeItemPersonRepository) Delete(_ context.Context, itemID, personID, role string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	for i, ip := range f.rows {
		if ip.ItemID == itemID && ip.PersonID == personID && ip.Role == role {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return ports.ErrNotFound
}

func (f *deletionFakeItemPersonRepository) List(_ context.Context, itemID, personID string, _ int, _ string) ([]*domain.ItemPerson, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var matched []*domain.ItemPerson
	for _, ip := range f.rows {
		if itemID != "" && ip.ItemID != itemID {
			continue
		}
		if personID != "" && ip.PersonID != personID {
			continue
		}
		matched = append(matched, ip)
	}
	return matched, "", nil
}

type deletionFakeMediaFileRepository struct {
	byID      map[string]*domain.MediaFile
	listErr   error
	deleteErr error
}

func (f *deletionFakeMediaFileRepository) Create(_ context.Context, m *domain.MediaFile) error {
	f.byID[m.ID] = m
	return nil
}

func (f *deletionFakeMediaFileRepository) Get(_ context.Context, id string) (*domain.MediaFile, error) {
	m, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return m, nil
}

func (f *deletionFakeMediaFileRepository) Update(_ context.Context, m *domain.MediaFile) error {
	f.byID[m.ID] = m
	return nil
}

func (f *deletionFakeMediaFileRepository) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *deletionFakeMediaFileRepository) List(_ context.Context, itemID string, _ int, _ string) ([]*domain.MediaFile, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var matched []*domain.MediaFile
	for _, m := range f.byID {
		if itemID != "" && m.ItemID != itemID {
			continue
		}
		matched = append(matched, m)
	}
	return matched, "", nil
}

// GetByHash is unused by ItemDeletionService's tests — present solely to
// satisfy ports.MediaFileRepository.
func (f *deletionFakeMediaFileRepository) GetByHash(context.Context, string, string, string, string) (*domain.MediaFile, error) {
	return nil, ports.ErrNotFound
}

type deletionFakeExternalIDRepository struct {
	rows      []*domain.ExternalID
	listErr   error
	deleteErr error
}

func (f *deletionFakeExternalIDRepository) Create(_ context.Context, e *domain.ExternalID) error {
	f.rows = append(f.rows, e)
	return nil
}

func (f *deletionFakeExternalIDRepository) Get(context.Context, domain.EntityType, string, string) (*domain.ExternalID, error) {
	return nil, ports.ErrNotFound
}

func (f *deletionFakeExternalIDRepository) GetByValue(_ context.Context, entityType domain.EntityType, source domain.ExternalIDSource, value string) (*domain.ExternalID, error) {
	for _, e := range f.rows {
		if e.EntityType == entityType && e.Source == source && e.Value == value {
			return e, nil
		}
	}
	return nil, ports.ErrNotFound
}

func (f *deletionFakeExternalIDRepository) Update(context.Context, *domain.ExternalID) error {
	return nil
}

func (f *deletionFakeExternalIDRepository) Delete(_ context.Context, entityType domain.EntityType, entityID, source string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	for i, e := range f.rows {
		if e.EntityType == entityType && e.EntityID == entityID && string(e.Source) == source {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return ports.ErrNotFound
}

func (f *deletionFakeExternalIDRepository) List(_ context.Context, entityType domain.EntityType, entityID string, _ int, _ string) ([]*domain.ExternalID, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var matched []*domain.ExternalID
	for _, e := range f.rows {
		if entityType != "" && e.EntityType != entityType {
			continue
		}
		if entityID != "" && e.EntityID != entityID {
			continue
		}
		matched = append(matched, e)
	}
	return matched, "", nil
}

type deletionFakeImageRepository struct {
	byID      map[string]*domain.Image
	listErr   error
	deleteErr error
}

func (f *deletionFakeImageRepository) Create(_ context.Context, img *domain.Image) error {
	f.byID[img.ID] = img
	return nil
}

func (f *deletionFakeImageRepository) Get(_ context.Context, id string) (*domain.Image, error) {
	img, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return img, nil
}

func (f *deletionFakeImageRepository) Update(_ context.Context, img *domain.Image) error {
	f.byID[img.ID] = img
	return nil
}

func (f *deletionFakeImageRepository) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *deletionFakeImageRepository) List(_ context.Context, ownerType, ownerID string, _ int, _ string) ([]*domain.Image, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var matched []*domain.Image
	for _, img := range f.byID {
		if ownerType != "" && img.OwnerType != ownerType {
			continue
		}
		if ownerID != "" && img.OwnerID != ownerID {
			continue
		}
		matched = append(matched, img)
	}
	return matched, "", nil
}

func newItemDeletionFixture() (*service.ItemDeletionService, *deletionFakeItemRepository, *deletionFakeItemPersonRepository, *deletionFakeMediaFileRepository, *deletionFakeExternalIDRepository, *deletionFakeImageRepository, *deletionFakeTagAssignmentRepository) {
	items := &deletionFakeItemRepository{byID: map[string]*domain.Item{"i1": {ID: "i1"}}}
	itemPeople := &deletionFakeItemPersonRepository{rows: []*domain.ItemPerson{
		{ItemID: "i1", PersonID: "p1", Role: "performer"},
		{ItemID: "i2", PersonID: "p2", Role: "performer"},
	}}
	mediaFiles := &deletionFakeMediaFileRepository{byID: map[string]*domain.MediaFile{
		"m1": {ID: "m1", ItemID: "i1"},
		"m2": {ID: "m2", ItemID: "i2"},
	}}
	externalIDs := &deletionFakeExternalIDRepository{rows: []*domain.ExternalID{
		{EntityType: domain.EntityTypeItem, EntityID: "i1", Source: domain.ExternalIDSourceTMDB, Value: "1"},
	}}
	images := &deletionFakeImageRepository{byID: map[string]*domain.Image{
		"img1": {ID: "img1", OwnerType: "item", OwnerID: "i1"},
	}}
	tagAssignments := &deletionFakeTagAssignmentRepository{rows: []*domain.TagAssignment{
		{TagID: "t1", EntityType: domain.EntityTypeItem, EntityID: "i1"},
	}}
	svc := service.NewItemDeletionService(items, itemPeople, mediaFiles, externalIDs, images, tagAssignments)
	return svc, items, itemPeople, mediaFiles, externalIDs, images, tagAssignments
}

func TestItemDeletionService_GetDeletionImpact(t *testing.T) {
	svc, _, _, _, _, _, _ := newItemDeletionFixture()

	impact, err := svc.GetDeletionImpact(context.Background(), "i1")
	if err != nil {
		t.Fatalf("GetDeletionImpact returned error: %v", err)
	}
	want := map[string]int{"item_person": 1, "media_file": 1, "external_id": 1, "image": 1, "tag_assignment": 1}
	for _, row := range impact.Impacts {
		if row.Blocking {
			t.Fatalf("Impacts row %q marked Blocking, want false — Item never blocks a delete", row.Kind)
		}
		if want[row.Kind] != row.Count {
			t.Fatalf("Impacts row %q Count = %d, want %d", row.Kind, row.Count, want[row.Kind])
		}
	}

	if _, err := svc.GetDeletionImpact(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetDeletionImpact on missing item returned %v, want ErrNotFound", err)
	}
}

func TestItemDeletionService_Delete(t *testing.T) {
	svc, items, itemPeople, mediaFiles, externalIDs, images, tagAssignments := newItemDeletionFixture()

	if err := svc.Delete(context.Background(), "i1", false); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if _, err := items.Get(context.Background(), "i1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("Delete did not remove the Item itself")
	}

	remainingItemPeople, _, _ := itemPeople.List(context.Background(), "", "", 10, "")
	if len(remainingItemPeople) != 1 || remainingItemPeople[0].ItemID != "i2" {
		t.Fatalf("Delete left %+v, want only the i2 credit untouched", remainingItemPeople)
	}

	remainingMediaFiles, _, _ := mediaFiles.List(context.Background(), "", 10, "")
	if len(remainingMediaFiles) != 1 || remainingMediaFiles[0].ItemID != "i2" {
		t.Fatalf("Delete left %+v, want only the i2 media file untouched", remainingMediaFiles)
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
}

func TestItemDeletionService_DeleteMissing(t *testing.T) {
	svc, _, _, _, _, _, _ := newItemDeletionFixture()

	if err := svc.Delete(context.Background(), "missing", false); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing item returned %v, want ErrNotFound", err)
	}
}

func TestItemDeletionService_Delete_PropagatesPortErrors(t *testing.T) {
	t.Run("itemPeople List error propagates", func(t *testing.T) {
		svc, _, itemPeople, _, _, _, _ := newItemDeletionFixture()
		itemPeople.listErr = errBoom
		if err := svc.Delete(context.Background(), "i1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("itemPeople Delete error propagates", func(t *testing.T) {
		svc, _, itemPeople, _, _, _, _ := newItemDeletionFixture()
		itemPeople.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "i1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("mediaFiles List error propagates", func(t *testing.T) {
		svc, _, _, mediaFiles, _, _, _ := newItemDeletionFixture()
		mediaFiles.listErr = errBoom
		if err := svc.Delete(context.Background(), "i1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("mediaFiles Delete error propagates", func(t *testing.T) {
		svc, _, _, mediaFiles, _, _, _ := newItemDeletionFixture()
		mediaFiles.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "i1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("externalIDs List error propagates", func(t *testing.T) {
		svc, _, _, _, externalIDs, _, _ := newItemDeletionFixture()
		externalIDs.listErr = errBoom
		if err := svc.Delete(context.Background(), "i1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("externalIDs Delete error propagates", func(t *testing.T) {
		svc, _, _, _, externalIDs, _, _ := newItemDeletionFixture()
		externalIDs.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "i1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("images List error propagates", func(t *testing.T) {
		svc, _, _, _, _, images, _ := newItemDeletionFixture()
		images.listErr = errBoom
		if err := svc.Delete(context.Background(), "i1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("images Delete error propagates", func(t *testing.T) {
		svc, _, _, _, _, images, _ := newItemDeletionFixture()
		images.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "i1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("tagAssignments List error propagates", func(t *testing.T) {
		svc, _, _, _, _, _, tagAssignments := newItemDeletionFixture()
		tagAssignments.listErr = errBoom
		if err := svc.Delete(context.Background(), "i1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("tagAssignments Delete error propagates", func(t *testing.T) {
		svc, _, _, _, _, _, tagAssignments := newItemDeletionFixture()
		tagAssignments.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "i1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("items Delete error propagates", func(t *testing.T) {
		svc, items, _, _, _, _, _ := newItemDeletionFixture()
		items.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "i1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("GetDeletionImpact propagates a List error", func(t *testing.T) {
		svc, _, _, _, _, images, _ := newItemDeletionFixture()
		images.listErr = errBoom
		if _, err := svc.GetDeletionImpact(context.Background(), "i1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})
}

func newItemDeletionBatchFixture() (*service.ItemDeletionService, *deletionFakeItemRepository, *deletionFakeTagAssignmentRepository) {
	items := &deletionFakeItemRepository{byID: map[string]*domain.Item{
		"i1": {ID: "i1"},
		"i2": {ID: "i2"},
		"i3": {ID: "i3"},
	}}
	itemPeople := &deletionFakeItemPersonRepository{rows: []*domain.ItemPerson{
		{ItemID: "i1", PersonID: "p1", Role: "performer"},
		{ItemID: "i2", PersonID: "p2", Role: "performer"},
		{ItemID: "i3", PersonID: "p3", Role: "performer"},
	}}
	mediaFiles := &deletionFakeMediaFileRepository{byID: map[string]*domain.MediaFile{}}
	externalIDs := &deletionFakeExternalIDRepository{}
	images := &deletionFakeImageRepository{byID: map[string]*domain.Image{}}
	tagAssignments := &deletionFakeTagAssignmentRepository{rows: []*domain.TagAssignment{
		{TagID: "t1", EntityType: domain.EntityTypeItem, EntityID: "i1"},
		{TagID: "t1", EntityType: domain.EntityTypeItem, EntityID: "i2"},
		{TagID: "t1", EntityType: domain.EntityTypeItem, EntityID: "i3"},
	}}
	svc := service.NewItemDeletionService(items, itemPeople, mediaFiles, externalIDs, images, tagAssignments)
	return svc, items, tagAssignments
}

func TestItemDeletionService_DeleteBatch(t *testing.T) {
	svc, items, tagAssignments := newItemDeletionBatchFixture()

	if err := svc.DeleteBatch(context.Background(), []string{"i1", "i2"}, false); err != nil {
		t.Fatalf("DeleteBatch returned error: %v", err)
	}

	if _, err := items.Get(context.Background(), "i1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("DeleteBatch did not remove i1")
	}
	if _, err := items.Get(context.Background(), "i2"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("DeleteBatch did not remove i2")
	}
	if _, err := items.Get(context.Background(), "i3"); err != nil {
		t.Fatalf("DeleteBatch removed i3, which wasn't in the batch: %v", err)
	}

	remaining, _, _ := tagAssignments.List(context.Background(), "", "", "", 10, "")
	if len(remaining) != 1 || remaining[0].EntityID != "i3" {
		t.Fatalf("DeleteBatch left %+v assignments, want only i3's untouched", remaining)
	}
}

func TestItemDeletionService_DeleteBatch_MissingIDFailsWithoutSideEffects(t *testing.T) {
	svc, items, tagAssignments := newItemDeletionBatchFixture()

	err := svc.DeleteBatch(context.Background(), []string{"i1", "missing", "i2"}, false)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("DeleteBatch with a missing id returned %v, want ErrNotFound", err)
	}

	if _, err := items.Get(context.Background(), "i1"); err != nil {
		t.Fatalf("DeleteBatch removed i1 despite failing: %v", err)
	}
	if _, err := items.Get(context.Background(), "i2"); err != nil {
		t.Fatalf("DeleteBatch removed i2 despite failing: %v", err)
	}
	remaining, _, _ := tagAssignments.List(context.Background(), "", "", "", 10, "")
	if len(remaining) != 3 {
		t.Fatalf("DeleteBatch unlinked assignments despite failing: %+v", remaining)
	}
}

func TestItemDeletionService_DeleteBatch_PropagatesPortErrors(t *testing.T) {
	t.Run("attachment unlink error propagates", func(t *testing.T) {
		svc, _, tagAssignments := newItemDeletionBatchFixture()
		tagAssignments.deleteErr = errBoom
		if err := svc.DeleteBatch(context.Background(), []string{"i1", "i2"}, false); !errors.Is(err, errBoom) {
			t.Fatalf("DeleteBatch returned %v, want errBoom", err)
		}
	})

	t.Run("items DeleteBatch error propagates", func(t *testing.T) {
		svc, items, _ := newItemDeletionBatchFixture()
		items.deleteBatchErr = errBoom
		if err := svc.DeleteBatch(context.Background(), []string{"i1", "i2"}, false); !errors.Is(err, errBoom) {
			t.Fatalf("DeleteBatch returned %v, want errBoom", err)
		}
	})
}
