package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type deletionFakeLibraryEntryRepository struct {
	byID           map[string]*domain.LibraryEntry
	listErr        error
	updateErr      error
	deleteErr      error
	deleteBatchErr error
}

func (f *deletionFakeLibraryEntryRepository) Create(_ context.Context, e *domain.LibraryEntry) error {
	f.byID[e.ID] = e
	return nil
}

func (f *deletionFakeLibraryEntryRepository) Get(_ context.Context, id string) (*domain.LibraryEntry, error) {
	e, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return e, nil
}

func (f *deletionFakeLibraryEntryRepository) Update(_ context.Context, e *domain.LibraryEntry) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.byID[e.ID] = e
	return nil
}

func (f *deletionFakeLibraryEntryRepository) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *deletionFakeLibraryEntryRepository) DeleteBatch(_ context.Context, ids []string) error {
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

func (f *deletionFakeLibraryEntryRepository) List(_ context.Context, kind domain.Kind, parentID string, _ int, _ string) ([]*domain.LibraryEntry, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	var matched []*domain.LibraryEntry
	for _, e := range f.byID {
		if kind != "" && e.Kind != kind {
			continue
		}
		if parentID != "" && e.ParentID != parentID {
			continue
		}
		matched = append(matched, e)
	}
	return matched, "", nil
}

// newLibraryEntryDeletionFixture builds:
//
//	network1 (LibraryEntry)
//	  studio1 (LibraryEntry, ParentID=network1) — has group1, item-under-studio1
//	    group1 (Group, LibraryEntryID=studio1) — has item-in-group1 (GroupID=group1),
//	      rel1 (music.Release, GroupID=group1, LibraryEntryID=studio1)
//	  studio2 (LibraryEntry, ParentID=network1) — no groups/items, only attachments
//
// network1 itself also carries an EntryPerson/ExternalID/Image/TagAssignment
// to prove attachment unlink happens regardless of cascade.
func newLibraryEntryDeletionFixture() (
	*service.LibraryEntryDeletionService,
	*deletionFakeLibraryEntryRepository,
	*deletionFakeGroupRepository,
	*deletionFakeItemRepositoryFiltered,
	*deletionFakeEntryPersonRepository,
	*deletionFakeExternalIDRepository,
	*deletionFakeImageRepository,
	*deletionFakeTagAssignmentRepository,
	*fakeMusicReleaseRepository,
) {
	libraryEntries := &deletionFakeLibraryEntryRepository{byID: map[string]*domain.LibraryEntry{
		"network1": {ID: "network1", Kind: domain.KindNetwork},
		"studio1":  {ID: "studio1", Kind: domain.KindStudio, ParentID: "network1"},
		"studio2":  {ID: "studio2", Kind: domain.KindStudio, ParentID: "network1"},
	}}
	groups := &deletionFakeGroupRepository{byID: map[string]*domain.Group{
		"group1": {ID: "group1", LibraryEntryID: "studio1"},
	}}
	items := &deletionFakeItemRepositoryFiltered{byID: map[string]*domain.Item{
		"item-studio1": {ID: "item-studio1", LibraryEntryID: "studio1"},
		"item-group1":  {ID: "item-group1", LibraryEntryID: "studio1", GroupID: "group1"},
		"trk-rel1":     {ID: "trk-rel1", LibraryEntryID: "studio1", GroupID: "group1", ContentType: domain.ContentTypeMusic, Metadata: map[string]any{"release_id": "rel1"}},
	}}
	entryPeople := &deletionFakeEntryPersonRepository{rows: []*domain.EntryPerson{
		{LibraryEntryID: "network1", PersonID: "p1", Role: "owner"},
	}}
	externalIDs := &deletionFakeExternalIDRepository{rows: []*domain.ExternalID{
		{EntityType: domain.EntityTypeLibraryEntry, EntityID: "network1", Source: domain.ExternalIDSourceStashDB, Value: "1"},
	}}
	images := &deletionFakeImageRepository{byID: map[string]*domain.Image{
		"img1": {ID: "img1", OwnerType: "library_entry", OwnerID: "network1"},
	}}
	tagAssignments := &deletionFakeTagAssignmentRepository{rows: []*domain.TagAssignment{
		{TagID: "t1", EntityType: domain.EntityTypeLibraryEntry, EntityID: "network1"},
	}}
	releases := newFakeMusicReleaseRepository()
	rel := validRelease("rel1")
	rel.GroupID = "group1"
	rel.LibraryEntryID = "studio1"
	if err := releases.Create(context.Background(), rel); err != nil {
		panic(err)
	}
	releases.tracksByRelease["rel1"] = []*domain.Item{items.byID["trk-rel1"]}

	musicReleaseDeletion := service.NewMusicReleaseDeletionService(releases, items)
	groupDeletion := service.NewGroupDeletionService(groups, items, externalIDs, images, tagAssignments, releases, musicReleaseDeletion)
	itemDeletion := service.NewItemDeletionService(
		items,
		&deletionFakeItemPersonRepository{},
		&deletionFakeMediaFileRepository{byID: map[string]*domain.MediaFile{}},
		externalIDs,
		images,
		tagAssignments,
	)
	svc := service.NewLibraryEntryDeletionService(libraryEntries, groups, items, entryPeople, externalIDs, images, tagAssignments, releases, groupDeletion, itemDeletion)
	return svc, libraryEntries, groups, items, entryPeople, externalIDs, images, tagAssignments, releases
}

func TestLibraryEntryDeletionService_GetDeletionImpact(t *testing.T) {
	svc, _, _, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()

	impact, err := svc.GetDeletionImpact(context.Background(), "network1")
	if err != nil {
		t.Fatalf("GetDeletionImpact returned error: %v", err)
	}
	blocking := map[string]bool{}
	counts := map[string]int{}
	for _, row := range impact.Impacts {
		counts[row.Kind] = row.Count
		blocking[row.Kind] = row.Blocking
	}
	if counts["library_entry_child"] != 2 {
		t.Fatalf("library_entry_child Count = %d, want 2 (studio1, studio2)", counts["library_entry_child"])
	}
	if !blocking["group"] || !blocking["item"] {
		t.Fatal("group and item rows must be marked Blocking for LibraryEntry")
	}
	if blocking["library_entry_child"] || blocking["entry_person"] || blocking["external_id"] || blocking["image"] || blocking["tag_assignment"] {
		t.Fatal("only group and item should be marked Blocking")
	}

	// network1 itself has no direct Groups/Items (they belong to studio1),
	// so its own impact shows 0 for those, even though a cascade delete
	// would still need to walk into studio1's subtree.
	if counts["group"] != 0 || counts["item"] != 0 {
		t.Fatalf("network1's direct group/item counts = (%d, %d), want (0, 0) — they belong to studio1", counts["group"], counts["item"])
	}
	// Same reasoning for music_release — rel1's LibraryEntryID is studio1,
	// not network1.
	if counts["music_release"] != 0 {
		t.Fatalf("network1's music_release count = %d, want 0 — rel1 belongs to studio1", counts["music_release"])
	}
	if blocking["music_release"] {
		t.Fatal("music_release must not be marked Blocking — cleanup falls out of the Group cascade, not a direct block")
	}

	if _, err := svc.GetDeletionImpact(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetDeletionImpact on missing entry returned %v, want ErrNotFound", err)
	}
}

func TestLibraryEntryDeletionService_GetDeletionImpact_BlockingOnDirectGroupsItems(t *testing.T) {
	svc, _, _, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()

	impact, err := svc.GetDeletionImpact(context.Background(), "studio1")
	if err != nil {
		t.Fatalf("GetDeletionImpact returned error: %v", err)
	}
	counts := map[string]int{}
	for _, row := range impact.Impacts {
		counts[row.Kind] = row.Count
	}
	if counts["group"] != 1 {
		t.Fatalf("studio1 group Count = %d, want 1", counts["group"])
	}
	if counts["item"] != 3 {
		t.Fatalf("studio1 item Count = %d, want 3 (item-studio1, item-group1, and trk-rel1 — an Item's LibraryEntryID is always set, grouped or not)", counts["item"])
	}
	if counts["music_release"] != 1 {
		t.Fatalf("studio1 music_release Count = %d, want 1 (rel1)", counts["music_release"])
	}
}

func TestLibraryEntryDeletionService_Delete_UnlinkBlocksWhenGroupsOrItemsExist(t *testing.T) {
	svc, libraryEntries, _, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()

	err := svc.Delete(context.Background(), "studio1", false)
	if !errors.Is(err, ports.ErrDeletionBlocked) {
		t.Fatalf("Delete(studio1, cascade=false) returned %v, want ErrDeletionBlocked", err)
	}

	// Blocked means zero effect — studio1 must still exist.
	if _, err := libraryEntries.Get(context.Background(), "studio1"); err != nil {
		t.Fatalf("Delete left studio1 removed despite being blocked: %v", err)
	}
}

func TestLibraryEntryDeletionService_Delete_UnlinkDetachesChildrenWithNoGroupsOrItems(t *testing.T) {
	svc, libraryEntries, _, _, entryPeople, externalIDs, images, tagAssignments, _ := newLibraryEntryDeletionFixture()

	// network1 has no *direct* Groups/Items (studio1 does), so Unlink
	// should succeed: studio1 and studio2 get detached, network1 is
	// removed, and its attachment rows are unlinked.
	if err := svc.Delete(context.Background(), "network1", false); err != nil {
		t.Fatalf("Delete(network1, cascade=false) returned error: %v", err)
	}

	if _, err := libraryEntries.Get(context.Background(), "network1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("Delete did not remove network1 itself")
	}

	studio1, err := libraryEntries.Get(context.Background(), "studio1")
	if err != nil {
		t.Fatalf("Delete removed studio1 entirely, want it detached but intact: %v", err)
	}
	if studio1.ParentID != "" {
		t.Fatalf("Delete left studio1 ParentID = %q, want empty (detached)", studio1.ParentID)
	}

	studio2, err := libraryEntries.Get(context.Background(), "studio2")
	if err != nil {
		t.Fatalf("Delete removed studio2 entirely, want it detached but intact: %v", err)
	}
	if studio2.ParentID != "" {
		t.Fatalf("Delete left studio2 ParentID = %q, want empty (detached)", studio2.ParentID)
	}

	remainingEntryPeople, _, _ := entryPeople.List(context.Background(), "", "", 10, "")
	if len(remainingEntryPeople) != 0 {
		t.Fatalf("Delete left %+v entry-person rows, want none", remainingEntryPeople)
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

func TestLibraryEntryDeletionService_Delete_CascadeDeletesEntireSubtree(t *testing.T) {
	svc, libraryEntries, groups, items, entryPeople, externalIDs, _, _, releases := newLibraryEntryDeletionFixture()

	if err := svc.Delete(context.Background(), "network1", true); err != nil {
		t.Fatalf("Delete(network1, cascade=true) returned error: %v", err)
	}

	for _, id := range []string{"network1", "studio1", "studio2"} {
		if _, err := libraryEntries.Get(context.Background(), id); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("cascade Delete did not remove LibraryEntry %q", id)
		}
	}
	if _, err := groups.Get(context.Background(), "group1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("cascade Delete did not remove group1")
	}
	// rel1 is removed transitively — LibraryEntry's cascade recurses into
	// GroupDeletionService, whose own Unlink deletes every Release under
	// group1. See docs/adr/0021-music-domain-model.md's "Ripple effects"
	// section.
	if _, err := releases.Get(context.Background(), "rel1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("cascade Delete did not remove rel1")
	}
	for _, id := range []string{"item-studio1", "item-group1", "trk-rel1"} {
		if _, err := items.Get(context.Background(), id); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("cascade Delete did not remove Item %q", id)
		}
	}

	remainingEntryPeople, _, _ := entryPeople.List(context.Background(), "", "", 10, "")
	if len(remainingEntryPeople) != 0 {
		t.Fatalf("cascade Delete left %+v entry-person rows, want none", remainingEntryPeople)
	}
	remainingExternalIDs, _, _ := externalIDs.List(context.Background(), "", "", 10, "")
	if len(remainingExternalIDs) != 0 {
		t.Fatalf("cascade Delete left %+v external IDs, want none", remainingExternalIDs)
	}
}

func TestLibraryEntryDeletionService_DeleteMissing(t *testing.T) {
	svc, _, _, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()

	if err := svc.Delete(context.Background(), "missing", false); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing entry returned %v, want ErrNotFound", err)
	}
}

func TestLibraryEntryDeletionService_GetDeletionImpact_PropagatesPortErrors(t *testing.T) {
	t.Run("children List error propagates", func(t *testing.T) {
		svc, libraryEntries, _, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		libraryEntries.listErr = errBoom
		if _, err := svc.GetDeletionImpact(context.Background(), "network1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})

	t.Run("groups List error propagates", func(t *testing.T) {
		svc, _, groups, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		groups.listErr = errBoom
		if _, err := svc.GetDeletionImpact(context.Background(), "network1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})

	t.Run("items List error propagates", func(t *testing.T) {
		svc, _, _, items, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		items.listErr = errBoom
		if _, err := svc.GetDeletionImpact(context.Background(), "network1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})

	t.Run("entryPeople List error propagates", func(t *testing.T) {
		svc, _, _, _, entryPeople, _, _, _, _ := newLibraryEntryDeletionFixture()
		entryPeople.listErr = errBoom
		if _, err := svc.GetDeletionImpact(context.Background(), "network1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})

	t.Run("externalIDs List error propagates", func(t *testing.T) {
		svc, _, _, _, _, externalIDs, _, _, _ := newLibraryEntryDeletionFixture()
		externalIDs.listErr = errBoom
		if _, err := svc.GetDeletionImpact(context.Background(), "network1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})

	t.Run("images List error propagates", func(t *testing.T) {
		svc, _, _, _, _, _, images, _, _ := newLibraryEntryDeletionFixture()
		images.listErr = errBoom
		if _, err := svc.GetDeletionImpact(context.Background(), "network1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})

	t.Run("tagAssignments List error propagates", func(t *testing.T) {
		svc, _, _, _, _, _, _, tagAssignments, _ := newLibraryEntryDeletionFixture()
		tagAssignments.listErr = errBoom
		if _, err := svc.GetDeletionImpact(context.Background(), "network1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})

	t.Run("musicReleases ListByEntry error propagates", func(t *testing.T) {
		svc, _, _, _, _, _, _, _, releases := newLibraryEntryDeletionFixture()
		releases.listByEntryErr = errBoom
		if _, err := svc.GetDeletionImpact(context.Background(), "network1"); !errors.Is(err, errBoom) {
			t.Fatalf("GetDeletionImpact returned %v, want errBoom", err)
		}
	})
}

func TestLibraryEntryDeletionService_Delete_Unlink_PropagatesPortErrors(t *testing.T) {
	t.Run("groups List error propagates from the blocking check", func(t *testing.T) {
		svc, _, groups, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		groups.listErr = errBoom
		if err := svc.Delete(context.Background(), "studio2", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("items List error propagates from the blocking check", func(t *testing.T) {
		svc, _, _, items, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		items.listErr = errBoom
		if err := svc.Delete(context.Background(), "studio2", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("libraryEntries List error propagates from detachChildren", func(t *testing.T) {
		svc, libraryEntries, _, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		libraryEntries.listErr = errBoom
		if err := svc.Delete(context.Background(), "studio2", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("libraryEntries Update error propagates from detachChildren", func(t *testing.T) {
		svc, libraryEntries, _, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		libraryEntries.updateErr = errBoom
		// network1 has no direct groups/items (they belong to studio1), so
		// Unlink proceeds past the blocking check into detachChildren,
		// which has two children (studio1, studio2) to Update.
		if err := svc.Delete(context.Background(), "network1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("entryPeople List error propagates from unlinkAttachments", func(t *testing.T) {
		svc, _, _, _, entryPeople, _, _, _, _ := newLibraryEntryDeletionFixture()
		entryPeople.listErr = errBoom
		if err := svc.Delete(context.Background(), "network1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("entryPeople Delete error propagates from unlinkAttachments", func(t *testing.T) {
		svc, _, _, _, entryPeople, _, _, _, _ := newLibraryEntryDeletionFixture()
		entryPeople.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "network1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("externalIDs List error propagates from unlinkAttachments", func(t *testing.T) {
		svc, _, _, _, _, externalIDs, _, _, _ := newLibraryEntryDeletionFixture()
		externalIDs.listErr = errBoom
		if err := svc.Delete(context.Background(), "network1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("externalIDs Delete error propagates from unlinkAttachments", func(t *testing.T) {
		svc, _, _, _, _, externalIDs, _, _, _ := newLibraryEntryDeletionFixture()
		externalIDs.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "network1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("images List error propagates from unlinkAttachments", func(t *testing.T) {
		svc, _, _, _, _, _, images, _, _ := newLibraryEntryDeletionFixture()
		images.listErr = errBoom
		if err := svc.Delete(context.Background(), "network1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("images Delete error propagates from unlinkAttachments", func(t *testing.T) {
		svc, _, _, _, _, _, images, _, _ := newLibraryEntryDeletionFixture()
		images.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "network1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("tagAssignments List error propagates from unlinkAttachments", func(t *testing.T) {
		svc, _, _, _, _, _, _, tagAssignments, _ := newLibraryEntryDeletionFixture()
		tagAssignments.listErr = errBoom
		if err := svc.Delete(context.Background(), "network1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("tagAssignments Delete error propagates from unlinkAttachments", func(t *testing.T) {
		svc, _, _, _, _, _, _, tagAssignments, _ := newLibraryEntryDeletionFixture()
		tagAssignments.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "network1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("libraryEntries Delete error propagates from the final delete", func(t *testing.T) {
		svc, libraryEntries, _, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		libraryEntries.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "studio2", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})
}

func TestLibraryEntryDeletionService_Delete_Cascade_PropagatesPortErrors(t *testing.T) {
	t.Run("children List error propagates from cascadeDeleteDescendants", func(t *testing.T) {
		svc, libraryEntries, _, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		libraryEntries.listErr = errBoom
		if err := svc.Delete(context.Background(), "network1", true); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("recursive child Delete error propagates", func(t *testing.T) {
		svc, _, groups, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		// group1 belongs to studio1, a child of network1 — failing its
		// deletion deep inside the recursive Delete(studio1, true) call
		// must surface all the way back up through network1's cascade.
		groups.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "network1", true); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("groupDeletion.Delete error propagates", func(t *testing.T) {
		svc, _, groups, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		groups.deleteErr = errBoom
		// Cascade directly on studio1 (which owns group1) isolates the
		// groupDeletion.Delete error-check line from the recursive-child
		// case above.
		if err := svc.Delete(context.Background(), "studio1", true); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("itemDeletion.Delete error propagates", func(t *testing.T) {
		svc, _, _, items, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		items.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "studio1", true); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("libraryEntries Delete error propagates from the final delete", func(t *testing.T) {
		svc, libraryEntries, _, _, _, _, _, _, _ := newLibraryEntryDeletionFixture()
		libraryEntries.deleteErr = errBoom
		if err := svc.Delete(context.Background(), "studio2", true); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})
}

// newLibraryEntryDeletionBatchFixture builds three independent, top-level
// LibraryEntries: e1 and e2 are clean (no Groups/Items), e3 has a Group
// (g1) with an Item (i1) under it — the one row whose single-row Delete
// precondition fails without cascade=true.
func newLibraryEntryDeletionBatchFixture() (
	*service.LibraryEntryDeletionService,
	*deletionFakeLibraryEntryRepository,
	*deletionFakeGroupRepository,
	*deletionFakeItemRepositoryFiltered,
) {
	libraryEntries := &deletionFakeLibraryEntryRepository{byID: map[string]*domain.LibraryEntry{
		"e1": {ID: "e1", Kind: domain.KindStudio},
		"e2": {ID: "e2", Kind: domain.KindStudio},
		"e3": {ID: "e3", Kind: domain.KindStudio},
	}}
	groups := &deletionFakeGroupRepository{byID: map[string]*domain.Group{
		"g1": {ID: "g1", LibraryEntryID: "e3"},
	}}
	items := &deletionFakeItemRepositoryFiltered{byID: map[string]*domain.Item{
		"i1": {ID: "i1", LibraryEntryID: "e3", GroupID: "g1"},
	}}
	entryPeople := &deletionFakeEntryPersonRepository{}
	externalIDs := &deletionFakeExternalIDRepository{}
	images := &deletionFakeImageRepository{byID: map[string]*domain.Image{}}
	tagAssignments := &deletionFakeTagAssignmentRepository{}
	releases := newFakeMusicReleaseRepository()

	musicReleaseDeletion := service.NewMusicReleaseDeletionService(releases, items)
	groupDeletion := service.NewGroupDeletionService(groups, items, externalIDs, images, tagAssignments, releases, musicReleaseDeletion)
	itemDeletion := service.NewItemDeletionService(
		items,
		&deletionFakeItemPersonRepository{},
		&deletionFakeMediaFileRepository{byID: map[string]*domain.MediaFile{}},
		externalIDs,
		images,
		tagAssignments,
	)
	svc := service.NewLibraryEntryDeletionService(libraryEntries, groups, items, entryPeople, externalIDs, images, tagAssignments, releases, groupDeletion, itemDeletion)
	return svc, libraryEntries, groups, items
}

func TestLibraryEntryDeletionService_DeleteBatch(t *testing.T) {
	svc, libraryEntries, _, _ := newLibraryEntryDeletionBatchFixture()

	if err := svc.DeleteBatch(context.Background(), []string{"e1", "e2"}, false); err != nil {
		t.Fatalf("DeleteBatch returned error: %v", err)
	}

	if _, err := libraryEntries.Get(context.Background(), "e1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("DeleteBatch did not remove e1")
	}
	if _, err := libraryEntries.Get(context.Background(), "e2"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("DeleteBatch did not remove e2")
	}
	if _, err := libraryEntries.Get(context.Background(), "e3"); err != nil {
		t.Fatalf("DeleteBatch removed e3, which wasn't in the batch: %v", err)
	}
}

// TestLibraryEntryDeletionService_DeleteBatch_BlockedRowFailsWholeBatch is
// the acceptance criterion from issue #654: a batch containing one row
// that would fail its own single-row Delete precondition (e3, which has a
// Group and an Item, cascade=false) must fail the entire batch — e1, which
// is otherwise perfectly deletable, must be left untouched too.
func TestLibraryEntryDeletionService_DeleteBatch_BlockedRowFailsWholeBatch(t *testing.T) {
	svc, libraryEntries, groups, items := newLibraryEntryDeletionBatchFixture()

	err := svc.DeleteBatch(context.Background(), []string{"e1", "e3"}, false)
	if !errors.Is(err, ports.ErrDeletionBlocked) {
		t.Fatalf("DeleteBatch returned %v, want ErrDeletionBlocked", err)
	}

	if _, err := libraryEntries.Get(context.Background(), "e1"); err != nil {
		t.Fatalf("DeleteBatch removed e1 despite the batch being blocked by e3: %v", err)
	}
	if _, err := libraryEntries.Get(context.Background(), "e3"); err != nil {
		t.Fatalf("DeleteBatch removed e3 despite being blocked: %v", err)
	}
	if _, err := groups.Get(context.Background(), "g1"); err != nil {
		t.Fatalf("DeleteBatch removed g1 despite the batch being blocked: %v", err)
	}
	if _, err := items.Get(context.Background(), "i1"); err != nil {
		t.Fatalf("DeleteBatch removed i1 despite the batch being blocked: %v", err)
	}
}

func TestLibraryEntryDeletionService_DeleteBatch_CascadeDeletesBlockedRowsToo(t *testing.T) {
	svc, libraryEntries, groups, items := newLibraryEntryDeletionBatchFixture()

	if err := svc.DeleteBatch(context.Background(), []string{"e1", "e3"}, true); err != nil {
		t.Fatalf("DeleteBatch(cascade=true) returned error: %v", err)
	}

	for _, id := range []string{"e1", "e3"} {
		if _, err := libraryEntries.Get(context.Background(), id); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("DeleteBatch(cascade=true) did not remove LibraryEntry %q", id)
		}
	}
	if _, err := groups.Get(context.Background(), "g1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("DeleteBatch(cascade=true) did not cascade-delete g1")
	}
	if _, err := items.Get(context.Background(), "i1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("DeleteBatch(cascade=true) did not cascade-delete i1")
	}
	if _, err := libraryEntries.Get(context.Background(), "e2"); err != nil {
		t.Fatalf("DeleteBatch removed e2, which wasn't in the batch: %v", err)
	}
}

func TestLibraryEntryDeletionService_DeleteBatch_MissingIDFailsWithoutSideEffects(t *testing.T) {
	svc, libraryEntries, _, _ := newLibraryEntryDeletionBatchFixture()

	err := svc.DeleteBatch(context.Background(), []string{"e1", "missing", "e2"}, false)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("DeleteBatch with a missing id returned %v, want ErrNotFound", err)
	}

	if _, err := libraryEntries.Get(context.Background(), "e1"); err != nil {
		t.Fatalf("DeleteBatch removed e1 despite failing: %v", err)
	}
	if _, err := libraryEntries.Get(context.Background(), "e2"); err != nil {
		t.Fatalf("DeleteBatch removed e2 despite failing: %v", err)
	}
}

func TestLibraryEntryDeletionService_DeleteBatch_PropagatesPortErrors(t *testing.T) {
	t.Run("groups List error propagates from the batch blocking check", func(t *testing.T) {
		svc, _, groups, _ := newLibraryEntryDeletionBatchFixture()
		groups.listErr = errBoom
		if err := svc.DeleteBatch(context.Background(), []string{"e1", "e2"}, false); !errors.Is(err, errBoom) {
			t.Fatalf("DeleteBatch returned %v, want errBoom", err)
		}
	})

	t.Run("libraryEntries DeleteBatch error propagates", func(t *testing.T) {
		svc, libraryEntries, _, _ := newLibraryEntryDeletionBatchFixture()
		libraryEntries.deleteBatchErr = errBoom
		if err := svc.DeleteBatch(context.Background(), []string{"e1", "e2"}, false); !errors.Is(err, errBoom) {
			t.Fatalf("DeleteBatch returned %v, want errBoom", err)
		}
	})
}
