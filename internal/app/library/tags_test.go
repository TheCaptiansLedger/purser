package library_test

import (
	"context"
	"database/sql"
	"fmt"
	"purser/internal/app/library"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// ── Tag repo mock ─────────────────────────────────────────────────────────────

type mockTagRepo struct {
	tags          map[string]*domain.Tag // keyed by ID
	groupTags     map[string][]string    // groupID → []tagID
	addGroupCalls int
	saveErr       error
	getErr        error
}

func newMockTagRepo() *mockTagRepo {
	return &mockTagRepo{
		tags:      make(map[string]*domain.Tag),
		groupTags: make(map[string][]string),
	}
}

func (m *mockTagRepo) Get(_ context.Context, id string) (*domain.Tag, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	t, ok := m.tags[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return t, nil
}

func (m *mockTagRepo) List(_ context.Context, f ports.TagFilter) ([]*domain.Tag, error) {
	var out []*domain.Tag
	for _, t := range m.tags {
		if f.Key != "" && t.Key != f.Key {
			continue
		}
		if f.Scope != "" && t.Scope != f.Scope {
			continue
		}
		if f.Value != "" && t.Value != f.Value {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (m *mockTagRepo) Save(_ context.Context, t *domain.Tag) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	if t.ID == "" {
		t.ID = fmt.Sprintf("tag-%d", len(m.tags)+1)
	}
	m.tags[t.ID] = t
	return nil
}

func (m *mockTagRepo) Delete(_ context.Context, id string) error {
	delete(m.tags, id)
	return nil
}

func (m *mockTagRepo) AddGroupTag(_ context.Context, groupID, tagID string) error {
	m.addGroupCalls++
	for _, existing := range m.groupTags[groupID] {
		if existing == tagID {
			return nil
		}
	}
	m.groupTags[groupID] = append(m.groupTags[groupID], tagID)
	return nil
}

func (m *mockTagRepo) RemoveGroupTag(_ context.Context, groupID, tagID string) error {
	existing := m.groupTags[groupID]
	updated := existing[:0]
	for _, id := range existing {
		if id != tagID {
			updated = append(updated, id)
		}
	}
	m.groupTags[groupID] = updated
	return nil
}

func newTagSvc(entries *mockEntryRepo, groups *mockGroupRepo, items *mockItemRepo, tags *mockTagRepo) *library.Service {
	return library.New(entries, groups, items, newMockPersonRepo(), tags)
}

// ── FindOrCreateTag ───────────────────────────────────────────────────────────

func TestFindOrCreateTag_CreatesOnFirstCall(t *testing.T) {
	tagRepo := newMockTagRepo()
	svc := newTagSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo(), tagRepo)
	ctx := context.Background()

	tag, err := svc.FindOrCreateTag(ctx, domain.TagKeyGenre, "Rock", domain.TagScopeMetadata)
	if err != nil {
		t.Fatalf("FindOrCreateTag: %v", err)
	}
	if tag.ID == "" {
		t.Error("tag ID should be set after create")
	}
	if len(tagRepo.tags) != 1 {
		t.Errorf("tag repo has %d tags, want 1", len(tagRepo.tags))
	}
}

func TestFindOrCreateTag_Idempotent(t *testing.T) {
	tagRepo := newMockTagRepo()
	svc := newTagSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo(), tagRepo)
	ctx := context.Background()

	first, err := svc.FindOrCreateTag(ctx, domain.TagKeyGenre, "Rock", domain.TagScopeMetadata)
	if err != nil {
		t.Fatalf("first FindOrCreateTag: %v", err)
	}
	second, err := svc.FindOrCreateTag(ctx, domain.TagKeyGenre, "Rock", domain.TagScopeMetadata)
	if err != nil {
		t.Fatalf("second FindOrCreateTag: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("second call returned different ID: %q vs %q", second.ID, first.ID)
	}
	if len(tagRepo.tags) != 1 {
		t.Errorf("tag repo has %d tags, want exactly 1 (no duplicate created)", len(tagRepo.tags))
	}
}

func TestFindOrCreateTag_CaseInsensitive(t *testing.T) {
	tagRepo := newMockTagRepo()
	svc := newTagSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo(), tagRepo)
	ctx := context.Background()

	first, err := svc.FindOrCreateTag(ctx, domain.TagKeyGenre, "Rock", domain.TagScopeMetadata)
	if err != nil {
		t.Fatalf("FindOrCreateTag: %v", err)
	}
	second, err := svc.FindOrCreateTag(ctx, domain.TagKeyGenre, "rock", domain.TagScopeMetadata)
	if err != nil {
		t.Fatalf("FindOrCreateTag (lower): %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("case variant created a duplicate: %q vs %q", first.ID, second.ID)
	}
	if len(tagRepo.tags) != 1 {
		t.Errorf("tag repo has %d tags, want 1", len(tagRepo.tags))
	}
}

// ── TagGroup ──────────────────────────────────────────────────────────────────

func TestTagGroup_AttachesTag(t *testing.T) {
	tagRepo := newMockTagRepo()
	svc := newTagSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo(), tagRepo)
	ctx := context.Background()

	if err := svc.TagGroup(ctx, "group-1", "tag-1"); err != nil {
		t.Fatalf("TagGroup: %v", err)
	}
	if tagRepo.addGroupCalls != 1 {
		t.Errorf("AddGroupTag calls = %d, want 1", tagRepo.addGroupCalls)
	}
}

func TestTagGroup_NoOpOnDuplicate(t *testing.T) {
	tagRepo := newMockTagRepo()
	svc := newTagSvc(newMockEntryRepo(), newMockGroupRepo(), newMockItemRepo(), tagRepo)
	ctx := context.Background()

	svc.TagGroup(ctx, "group-1", "tag-1") //nolint:errcheck
	svc.TagGroup(ctx, "group-1", "tag-1") //nolint:errcheck

	if len(tagRepo.groupTags["group-1"]) != 1 {
		t.Errorf("group has %d tags, want 1 after duplicate attach", len(tagRepo.groupTags["group-1"]))
	}
}

// ── TagItem ───────────────────────────────────────────────────────────────────

func TestTagItem_AttachesTag(t *testing.T) {
	items := newMockItemRepo()
	tagRepo := newMockTagRepo()
	svc := newTagSvc(newMockEntryRepo(), newMockGroupRepo(), items, tagRepo)
	ctx := context.Background()

	item := &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: "entry-1", Title: "Scene"}
	svc.CreateItem(ctx, item) //nolint:errcheck

	tag := &domain.Tag{ID: "tag-1", Key: domain.TagKeyGenre, Value: "outdoor", Scope: domain.TagScopeMetadata}
	tagRepo.tags["tag-1"] = tag

	if err := svc.TagItem(ctx, item.ID, "tag-1"); err != nil {
		t.Fatalf("TagItem: %v", err)
	}
	saved := items.data[item.ID]
	if len(saved.Tags) != 1 {
		t.Errorf("item has %d tags, want 1", len(saved.Tags))
	}
}

func TestTagItem_NoOpOnDuplicate(t *testing.T) {
	items := newMockItemRepo()
	tagRepo := newMockTagRepo()
	svc := newTagSvc(newMockEntryRepo(), newMockGroupRepo(), items, tagRepo)
	ctx := context.Background()

	item := &domain.Item{ContentType: domain.ContentTypeAdult, LibraryEntryID: "entry-1", Title: "Scene"}
	svc.CreateItem(ctx, item) //nolint:errcheck

	tag := &domain.Tag{ID: "tag-1", Key: domain.TagKeyGenre, Value: "outdoor", Scope: domain.TagScopeMetadata}
	tagRepo.tags["tag-1"] = tag

	svc.TagItem(ctx, item.ID, "tag-1") //nolint:errcheck
	svc.TagItem(ctx, item.ID, "tag-1") //nolint:errcheck

	saved := items.data[item.ID]
	if len(saved.Tags) != 1 {
		t.Errorf("item has %d tags after duplicate attach, want 1", len(saved.Tags))
	}
}

// ── TagEntry ──────────────────────────────────────────────────────────────────

func TestTagEntry_AttachesTag(t *testing.T) {
	entries := newMockEntryRepo()
	tagRepo := newMockTagRepo()
	svc := newTagSvc(entries, newMockGroupRepo(), newMockItemRepo(), tagRepo)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "Studio A"}
	svc.CreateEntry(ctx, entry) //nolint:errcheck

	tag := &domain.Tag{ID: "tag-1", Key: domain.TagKeyGenre, Value: "action", Scope: domain.TagScopeMetadata}
	tagRepo.tags["tag-1"] = tag

	if err := svc.TagEntry(ctx, entry.ID, "tag-1"); err != nil {
		t.Fatalf("TagEntry: %v", err)
	}
	saved := entries.data[entry.ID]
	if len(saved.Tags) != 1 {
		t.Errorf("entry has %d tags, want 1", len(saved.Tags))
	}
}

func TestTagEntry_NoOpOnDuplicate(t *testing.T) {
	entries := newMockEntryRepo()
	tagRepo := newMockTagRepo()
	svc := newTagSvc(entries, newMockGroupRepo(), newMockItemRepo(), tagRepo)
	ctx := context.Background()

	entry := &domain.LibraryEntry{ContentType: domain.ContentTypeAdult, Kind: domain.KindStudio, Name: "Studio A"}
	svc.CreateEntry(ctx, entry) //nolint:errcheck

	tag := &domain.Tag{ID: "tag-1", Key: domain.TagKeyGenre, Value: "action", Scope: domain.TagScopeMetadata}
	tagRepo.tags["tag-1"] = tag

	svc.TagEntry(ctx, entry.ID, "tag-1") //nolint:errcheck
	svc.TagEntry(ctx, entry.ID, "tag-1") //nolint:errcheck

	saved := entries.data[entry.ID]
	if len(saved.Tags) != 1 {
		t.Errorf("entry has %d tags after duplicate attach, want 1", len(saved.Tags))
	}
}
