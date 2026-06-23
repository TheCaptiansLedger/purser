package library

import (
	"context"
	"fmt"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"

	"github.com/google/uuid"
)

// FindOrCreateTag returns the existing tag matching key+value (case-insensitive)+scope,
// or creates it. Safe to call concurrently — INSERT OR IGNORE semantics in the adapter.
func (s *Service) FindOrCreateTag(ctx context.Context, key domain.TagKey, value string, scope domain.TagScope) (*domain.Tag, error) {
	existing, err := s.tags.List(ctx, ports.TagFilter{Key: key, Scope: scope})
	if err != nil {
		return nil, fmt.Errorf("find or create tag: %w", err)
	}
	lower := strings.ToLower(value)
	for _, t := range existing {
		if strings.ToLower(t.Value) == lower {
			return t, nil
		}
	}
	t := &domain.Tag{ID: uuid.New().String(), Key: key, Value: value, Scope: scope}
	if err := s.tags.Save(ctx, t); err != nil {
		return nil, fmt.Errorf("find or create tag: save: %w", err)
	}
	return t, nil
}

// TagGroup attaches a tag to a group. No-op if already attached.
func (s *Service) TagGroup(ctx context.Context, groupID, tagID string) error {
	if err := s.tags.AddGroupTag(ctx, groupID, tagID); err != nil {
		return fmt.Errorf("tag group: %w", err)
	}
	return nil
}

// TagItem attaches a tag to an item. No-op if already attached.
func (s *Service) TagItem(ctx context.Context, itemID, tagID string) error {
	item, err := s.GetItem(ctx, itemID)
	if err != nil {
		return err
	}
	for _, t := range item.Tags {
		if t.ID == tagID {
			return nil
		}
	}
	tag, err := s.tags.Get(ctx, tagID)
	if err != nil {
		return fmt.Errorf("tag item: get tag: %w", err)
	}
	item.Tags = append(item.Tags, *tag)
	return s.items.Save(ctx, item)
}

// TagEntry attaches a tag to a library entry. No-op if already attached.
func (s *Service) TagEntry(ctx context.Context, entryID, tagID string) error {
	entry, err := s.GetEntry(ctx, entryID)
	if err != nil {
		return err
	}
	for _, t := range entry.Tags {
		if t.ID == tagID {
			return nil
		}
	}
	tag, err := s.tags.Get(ctx, tagID)
	if err != nil {
		return fmt.Errorf("tag entry: get tag: %w", err)
	}
	entry.Tags = append(entry.Tags, *tag)
	return s.entries.Save(ctx, entry)
}
