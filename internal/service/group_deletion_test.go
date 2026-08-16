package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type deletionFakeGroupRepository struct {
	byID      map[string]*domain.Group
	listErr   error
	deleteErr error
}

func (f *deletionFakeGroupRepository) Create(_ context.Context, g *domain.Group) error {
	f.byID[g.ID] = g
	return nil
}

func (f *deletionFakeGroupRepository) Get(_ context.Context, id string) (*domain.Group, error) {
	g, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return g, nil
}

func (f *deletionFakeGroupRepository) Update(_ context.Context, g *domain.Group) error {
	f.byID[g.ID] = g
	return nil
}

func (f *deletionFakeGroupRepository) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *deletionFakeGroupRepository) List(_ context.Context, libraryEntryID string, _ int, _ string) ([]*domain.Group, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var matched []*domain.Group
	for _, g := range f.byID {
		if libraryEntryID != "" && g.LibraryEntryID != libraryEntryID {
			continue
		}
		matched = append(matched, g)
	}
	return matched, "", nil
}

// deletionFakeItemRepositoryFiltered supports the libraryEntryID/
// contentType/groupID filtering GroupDeletionService needs — distinct
// from deletionFakeItemRepository (item_deletion_test.go), which is a
// no-op List since ItemDeletionService only ever needs itemPeople/
// mediaFiles/etc. filtered by itemID, never Item itself filtered.
type deletionFakeItemRepositoryFiltered struct {
	byID           map[string]*domain.Item
	listErr        error
	updateErr      error
	deleteErr      error
	deleteBatchErr error
}

func (f *deletionFakeItemRepositoryFiltered) Create(_ context.Context, i *domain.Item) error {
	f.byID[i.ID] = i
	return nil
}

func (f *deletionFakeItemRepositoryFiltered) Get(_ context.Context, id string) (*domain.Item, error) {
	i, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return i, nil
}

func (f *deletionFakeItemRepositoryFiltered) Update(_ context.Context, i *domain.Item) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.byID[i.ID] = i
	return nil
}

func (f *deletionFakeItemRepositoryFiltered) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *deletionFakeItemRepositoryFiltered) DeleteBatch(_ context.Context, ids []string) error {
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

func (f *deletionFakeItemRepositoryFiltered) List(_ context.Context, libraryEntryID, contentType, groupID string, status domain.ItemStatus, _ int, _ string) ([]*domain.Item, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var matched []*domain.Item
	for _, i := range f.byID {
		if libraryEntryID != "" && i.LibraryEntryID != libraryEntryID {
			continue
		}
		if contentType != "" && string(i.ContentType) != contentType {
			continue
		}
		if groupID != "" && i.GroupID != groupID {
			continue
		}
		if status != "" && i.Status != status {
			continue
		}
		matched = append(matched, i)
	}
	return matched, "", nil
}

func newGroupDeletionFixture() (
	*service.GroupDeletionService,
	*deletionFakeGroupRepository,
	*deletionFakeItemRepositoryFiltered,
	*deletionFakeExternalIDRepository,
	*deletionFakeImageRepository,
	*deletionFakeTagAssignmentRepository,
	*fakeMusicReleaseRepository,
) {
	groups := &deletionFakeGroupRepository{byID: map[string]*domain.Group{"g1": {ID: "g1", LibraryEntryID: "e1"}}}
	items := &deletionFakeItemRepositoryFiltered{byID: map[string]*domain.Item{
		"i1":   {ID: "i1", LibraryEntryID: "e1", GroupID: "g1"},
		"i2":   {ID: "i2", LibraryEntryID: "e1", GroupID: "g1"},
		"i3":   {ID: "i3", LibraryEntryID: "e1", GroupID: "g2"},
		"trk1": {ID: "trk1", LibraryEntryID: "e1", GroupID: "g1", ContentType: domain.ContentTypeMusic, Metadata: map[string]any{"release_id": "rel1"}},
	}}
	externalIDs := &deletionFakeExternalIDRepository{rows: []*domain.ExternalID{
		{EntityType: domain.EntityTypeGroup, EntityID: "g1", Source: domain.ExternalIDSourceTMDB, Value: "1"},
	}}
	images := &deletionFakeImageRepository{byID: map[string]*domain.Image{
		"img1": {ID: "img1", OwnerType: "group", OwnerID: "g1"},
	}}
	tagAssignments := &deletionFakeTagAssignmentRepository{rows: []*domain.TagAssignment{
		{TagID: "t1", EntityType: domain.EntityTypeGroup, EntityID: "g1"},
	}}
	releases := newFakeMusicReleaseRepository()
	rel := validRelease("rel1")
	rel.GroupID = "g1"
	rel.LibraryEntryID = "e1"
	if err := releases.Create(context.Background(), rel); err != nil {
		panic(err)
	}
	releases.tracksByRelease["rel1"] = []*domain.Item{items.byID["trk1"]}

	musicReleaseDeletion := service.NewMusicReleaseDeletionService(releases, items)
	svc := service.NewGroupDeletionService(groups, items, externalIDs, images, tagAssignments, releases, musicReleaseDeletion)
	return svc, groups, items, externalIDs, images, tagAssignments, releases
}

func TestGroupDeletionService_GetDeletionImpact(t *testing.T) {
	svc, _, _, _, _, _, _ := newGroupDeletionFixture()

	impact, err := svc.GetDeletionImpact(context.Background(), "g1")
	if err != nil {
		t.Fatalf("GetDeletionImpact returned error: %v", err)
	}
	want := map[string]int{"item": 3, "external_id": 1, "image": 1, "tag_assignment": 1, "music_release": 1}
	for _, row := range impact.Impacts {
		if row.Blocking {
			t.Fatalf("Impacts row %q marked Blocking, want false — Group never blocks a delete", row.Kind)
		}
		if want[row.Kind] != row.Count {
			t.Fatalf("Impacts row %q Count = %d, want %d", row.Kind, row.Count, want[row.Kind])
		}
	}

	if _, err := svc.GetDeletionImpact(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetDeletionImpact on missing group returned %v, want ErrNotFound", err)
	}
}

func TestGroupDeletionService_Delete_DetachesItemsRatherThanDeletingThem(t *testing.T) {
	svc, groups, items, externalIDs, images, tagAssignments, _ := newGroupDeletionFixture()

	if err := svc.Delete(context.Background(), "g1", false); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if _, err := groups.Get(context.Background(), "g1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("Delete did not remove the Group itself")
	}

	// The two items that belonged to g1 must still exist, with the same
	// LibraryEntryID, just detached from the group — not deleted.
	i1, err := items.Get(context.Background(), "i1")
	if err != nil {
		t.Fatalf("Delete removed Item i1 entirely, want it detached but intact: %v", err)
	}
	if i1.GroupID != "" {
		t.Fatalf("Delete left Item i1 GroupID = %q, want empty (detached)", i1.GroupID)
	}
	if i1.LibraryEntryID != "e1" {
		t.Fatalf("Delete changed Item i1 LibraryEntryID to %q, want it untouched", i1.LibraryEntryID)
	}

	i2, err := items.Get(context.Background(), "i2")
	if err != nil {
		t.Fatalf("Delete removed Item i2 entirely, want it detached but intact: %v", err)
	}
	if i2.GroupID != "" {
		t.Fatalf("Delete left Item i2 GroupID = %q, want empty (detached)", i2.GroupID)
	}

	// i3 belonged to a different group (g2) and must be completely untouched.
	i3, err := items.Get(context.Background(), "i3")
	if err != nil {
		t.Fatalf("Delete affected an unrelated item: %v", err)
	}
	if i3.GroupID != "g2" {
		t.Fatalf("Delete changed Item i3 GroupID to %q, want it untouched (g2)", i3.GroupID)
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

// TestGroupDeletionService_Delete_DeletesMusicReleasesRatherThanDetachingThem
// proves the one referrer that behaves differently from Item: since
// music.Release.GroupID is required (unlike Item.GroupID), Group's Unlink
// deletes the referencing Release outright rather than clearing a field —
// and that delete itself goes through MusicReleaseDeletionService's own
// Unlink, clearing Metadata["release_id"] on the release's tracks rather
// than deleting them. See docs/adr/0021-music-domain-model.md's "Ripple
// effects" section.
func TestGroupDeletionService_Delete_DeletesMusicReleasesRatherThanDetachingThem(t *testing.T) {
	svc, _, items, _, _, _, releases := newGroupDeletionFixture()

	if err := svc.Delete(context.Background(), "g1", false); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if _, err := releases.Get(context.Background(), "rel1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("Delete did not remove the Group's Release (rel1)")
	}

	trk1, err := items.Get(context.Background(), "trk1")
	if err != nil {
		t.Fatalf("Delete removed the release's track entirely, want it detached but intact: %v", err)
	}
	if _, ok := trk1.Metadata["release_id"]; ok {
		t.Fatalf("Delete left track Metadata[release_id] = %v, want the key cleared", trk1.Metadata["release_id"])
	}
}

func TestGroupDeletionService_DeleteMissing(t *testing.T) {
	svc, _, _, _, _, _, _ := newGroupDeletionFixture()

	if err := svc.Delete(context.Background(), "missing", false); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing group returned %v, want ErrNotFound", err)
	}
}

func TestGroupDeletionService_Delete_PropagatesPortErrors(t *testing.T) {
	t.Run("items List error propagates", func(t *testing.T) {
		svc, _, items, _, _, _, _ := newGroupDeletionFixture()
		items.listErr = errBoom
		if err := svc.Delete(context.Background(), "g1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("items Update error propagates", func(t *testing.T) {
		svc, _, items, _, _, _, _ := newGroupDeletionFixture()
		items.updateErr = errBoom
		if err := svc.Delete(context.Background(), "g1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("externalIDs List error propagates", func(t *testing.T) {
		svc, _, _, externalIDs, _, _, _ := newGroupDeletionFixture()
		externalIDs.listErr = errBoom
		if err := svc.Delete(context.Background(), "g1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("externalIDs Delete error propagates", func(t *testing.T) {
		svc, _, _, externalIDs, _, _, _ := newGroupDeletionFixture()
		externalIDs.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "g1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("images List error propagates", func(t *testing.T) {
		svc, _, _, _, images, _, _ := newGroupDeletionFixture()
		images.listErr = errBoom
		if err := svc.Delete(context.Background(), "g1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("images Delete error propagates", func(t *testing.T) {
		svc, _, _, _, images, _, _ := newGroupDeletionFixture()
		images.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "g1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("tagAssignments List error propagates", func(t *testing.T) {
		svc, _, _, _, _, tagAssignments, _ := newGroupDeletionFixture()
		tagAssignments.listErr = errBoom
		if err := svc.Delete(context.Background(), "g1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("tagAssignments Delete error propagates", func(t *testing.T) {
		svc, _, _, _, _, tagAssignments, _ := newGroupDeletionFixture()
		tagAssignments.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "g1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("groups Delete error propagates", func(t *testing.T) {
		svc, groups, _, _, _, _, _ := newGroupDeletionFixture()
		groups.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "g1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("GetDeletionImpact propagates a List error", func(t *testing.T) {
		svc, _, items, _, _, _, _ := newGroupDeletionFixture()
		items.listErr = errBoom
		if _, err := svc.GetDeletionImpact(context.Background(), "g1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})

	t.Run("musicReleases ListByGroup error propagates", func(t *testing.T) {
		svc, _, _, _, _, _, releases := newGroupDeletionFixture()
		releases.listByGroupErr = errBoom
		if err := svc.Delete(context.Background(), "g1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("musicReleaseDeletion Delete error propagates", func(t *testing.T) {
		svc, _, _, _, _, _, releases := newGroupDeletionFixture()
		releases.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "g1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("GetDeletionImpact propagates a musicReleases List error", func(t *testing.T) {
		svc, _, _, _, _, _, releases := newGroupDeletionFixture()
		releases.listByGroupErr = errBoom
		if _, err := svc.GetDeletionImpact(context.Background(), "g1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})
}
