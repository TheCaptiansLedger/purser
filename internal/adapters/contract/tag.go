package contract

import (
	"context"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

func runTagContract(t *testing.T, s BackendSuite) { //nolint:cyclop
	t.Helper()

	t.Run("SaveGetDelete", func(t *testing.T) {
		ctx := context.Background()
		tag := &domain.Tag{Key: domain.TagKeyGeneral, Value: "contract-test-tag", Scope: domain.TagScopeUser}
		if err := s.Tags.Save(ctx, tag); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if tag.ID == "" {
			t.Fatal("Save must set ID")
		}
		got, err := s.Tags.Get(ctx, tag.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got.Value != "contract-test-tag" {
			t.Errorf("Value = %q, want contract-test-tag", got.Value)
		}
		if err := s.Tags.Delete(ctx, tag.ID); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err = s.Tags.Get(ctx, tag.ID)
		if !errs.IsNotFound(err) {
			t.Errorf("after Delete Get: want ErrNotFound, got %v", err)
		}
	})

	t.Run("ListByScopeFilter", func(t *testing.T) {
		ctx := context.Background()
		user := &domain.Tag{Key: domain.TagKeyGeneral, Value: "scope-user-tag", Scope: domain.TagScopeUser}
		meta := &domain.Tag{Key: domain.TagKeyGeneral, Value: "scope-meta-tag", Scope: domain.TagScopeMetadata}
		for _, tag := range []*domain.Tag{user, meta} {
			if err := s.Tags.Save(ctx, tag); err != nil {
				t.Fatalf("Save: %v", err)
			}
		}
		userTags, err := s.Tags.List(ctx, ports.TagFilter{Scope: domain.TagScopeUser})
		if err != nil {
			t.Fatalf("List by scope: %v", err)
		}
		for _, tag := range userTags {
			if tag.Scope != domain.TagScopeUser {
				t.Errorf("scope filter leaked non-user tag %q", tag.Value)
			}
		}
	})

	t.Run("ListByKeyFilter", func(t *testing.T) {
		ctx := context.Background()
		genre := &domain.Tag{Key: domain.TagKeyGenre, Value: "contract-genre", Scope: domain.TagScopeMetadata}
		general := &domain.Tag{Key: domain.TagKeyGeneral, Value: "contract-general-tag2", Scope: domain.TagScopeUser}
		for _, tag := range []*domain.Tag{genre, general} {
			if err := s.Tags.Save(ctx, tag); err != nil {
				t.Fatalf("Save: %v", err)
			}
		}
		genres, err := s.Tags.List(ctx, ports.TagFilter{Key: domain.TagKeyGenre})
		if err != nil {
			t.Fatalf("List by key: %v", err)
		}
		for _, tag := range genres {
			if tag.Key != domain.TagKeyGenre {
				t.Errorf("key filter leaked tag with key %q", tag.Key)
			}
		}
	})

	t.Run("AddAndRemoveGroupTag", func(t *testing.T) {
		ctx := context.Background()

		entry := &domain.LibraryEntry{
			ContentType: domain.ContentTypeTV,
			Kind:        domain.KindSeries,
			Name:        "Tag Contract Series",
			MonitorMode: domain.MonitorAll,
			Status:      domain.EntryStatusActive,
		}
		if err := s.LibraryEntries.Save(ctx, entry); err != nil {
			t.Fatalf("Save entry: %v", err)
		}
		group := &domain.Group{
			LibraryEntryID: entry.ID,
			Title:          "Tag Contract Season",
			Monitored:      true,
			MonitorMode:    domain.MonitorAll,
		}
		if err := s.Groups.Save(ctx, group); err != nil {
			t.Fatalf("Save group: %v", err)
		}

		tag := &domain.Tag{Key: domain.TagKeyGeneral, Value: "group-tag-contract", Scope: domain.TagScopeUser}
		if err := s.Tags.Save(ctx, tag); err != nil {
			t.Fatalf("Save tag: %v", err)
		}

		if err := s.Tags.AddGroupTag(ctx, group.ID, tag.ID); err != nil {
			t.Fatalf("AddGroupTag: %v", err)
		}
		tagged, err := s.Tags.List(ctx, ports.TagFilter{GroupID: group.ID})
		if err != nil {
			t.Fatalf("List by GroupID: %v", err)
		}
		if len(tagged) != 1 || tagged[0].ID != tag.ID {
			t.Errorf("after AddGroupTag: group tags = %d, want 1", len(tagged))
		}

		if err := s.Tags.RemoveGroupTag(ctx, group.ID, tag.ID); err != nil {
			t.Fatalf("RemoveGroupTag: %v", err)
		}
		untagged, err := s.Tags.List(ctx, ports.TagFilter{GroupID: group.ID})
		if err != nil {
			t.Fatalf("List after remove: %v", err)
		}
		if len(untagged) != 0 {
			t.Errorf("after RemoveGroupTag: group tags = %d, want 0", len(untagged))
		}
	})
}
