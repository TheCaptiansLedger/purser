package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// GroupDeletionService is Group's composing deletion service — the
// explicit multi-port exception
// docs/adr/0015-deletion-impact-and-composing-services.md carves out.
// internal/service/group.go stays single-port; this is genuinely the
// exception, not a reason to weaken that rule generally.
//
// Item.GroupID is Group's one structurally different referrer: it's
// optional (nullable), so unlike a required foreign key it can be
// detached rather than requiring the referrer itself to be deleted or the
// delete to be blocked — Delete clears GroupID on every Item under the
// group (keeping the Item itself, and its LibraryEntryID, untouched)
// rather than deleting those Items. ExternalID, Image, and TagAssignment
// are ordinary attachment rows and are deleted on Unlink as usual. Group
// never blocks a delete; cascade is accepted for API-shape consistency
// but has no effect here.
type GroupDeletionService struct {
	groups         ports.GroupRepository
	items          ports.ItemRepository
	externalIDs    ports.ExternalIDRepository
	images         ports.ImageRepository
	tagAssignments ports.TagAssignmentRepository
}

// NewGroupDeletionService constructs a GroupDeletionService backed by the
// given ports.
func NewGroupDeletionService(
	groups ports.GroupRepository,
	items ports.ItemRepository,
	externalIDs ports.ExternalIDRepository,
	images ports.ImageRepository,
	tagAssignments ports.TagAssignmentRepository,
) *GroupDeletionService {
	return &GroupDeletionService{
		groups:         groups,
		items:          items,
		externalIDs:    externalIDs,
		images:         images,
		tagAssignments: tagAssignments,
	}
}

// GetDeletionImpact returns what references the Group identified by id,
// or ports.ErrNotFound if the Group doesn't exist.
func (s *GroupDeletionService) GetDeletionImpact(ctx context.Context, id string) (*domain.DeletionImpact, error) {
	if _, err := s.groups.Get(ctx, id); err != nil {
		return nil, err
	}

	items, err := s.drainItems(ctx, id)
	if err != nil {
		return nil, err
	}
	externalIDs, err := s.drainExternalIDs(ctx, id)
	if err != nil {
		return nil, err
	}
	images, err := s.drainImages(ctx, id)
	if err != nil {
		return nil, err
	}
	tagAssignments, err := s.drainTagAssignments(ctx, id)
	if err != nil {
		return nil, err
	}

	return &domain.DeletionImpact{
		Impacts: []domain.DeletionImpactRow{
			{Kind: "item", Label: "Items (will be detached, not deleted)", Count: len(items)},
			{Kind: "external_id", Label: "External IDs", Count: len(externalIDs)},
			{Kind: "image", Label: "Images", Count: len(images)},
			{Kind: "tag_assignment", Label: "Tags", Count: len(tagAssignments)},
		},
	}, nil
}

// Delete removes the Group identified by id, detaching every Item under
// it (clearing GroupID, leaving the Item and its LibraryEntryID intact)
// and unlinking every attachment row. Returns ports.ErrNotFound if the
// Group doesn't exist.
func (s *GroupDeletionService) Delete(ctx context.Context, id string, _ bool) error {
	if _, err := s.groups.Get(ctx, id); err != nil {
		return err
	}
	if err := s.detachItems(ctx, id); err != nil {
		return err
	}
	if err := s.unlinkExternalIDs(ctx, id); err != nil {
		return err
	}
	if err := s.unlinkImages(ctx, id); err != nil {
		return err
	}
	if err := s.unlinkTagAssignments(ctx, id); err != nil {
		return err
	}
	return s.groups.Delete(ctx, id)
}

func (s *GroupDeletionService) detachItems(ctx context.Context, groupID string) error {
	items, err := s.drainItems(ctx, groupID)
	if err != nil {
		return err
	}
	for _, i := range items {
		i.GroupID = ""
		if err := s.items.Update(ctx, i); err != nil {
			return err
		}
	}
	return nil
}

func (s *GroupDeletionService) unlinkExternalIDs(ctx context.Context, groupID string) error {
	rows, err := s.drainExternalIDs(ctx, groupID)
	if err != nil {
		return err
	}
	for _, e := range rows {
		if err := s.externalIDs.Delete(ctx, e.EntityType, e.EntityID, string(e.Source)); err != nil {
			return err
		}
	}
	return nil
}

func (s *GroupDeletionService) unlinkImages(ctx context.Context, groupID string) error {
	rows, err := s.drainImages(ctx, groupID)
	if err != nil {
		return err
	}
	for _, img := range rows {
		if err := s.images.Delete(ctx, img.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *GroupDeletionService) unlinkTagAssignments(ctx context.Context, groupID string) error {
	rows, err := s.drainTagAssignments(ctx, groupID)
	if err != nil {
		return err
	}
	for _, ta := range rows {
		if err := s.tagAssignments.Delete(ctx, ta.TagID, ta.EntityType, ta.EntityID); err != nil {
			return err
		}
	}
	return nil
}

func (s *GroupDeletionService) drainItems(ctx context.Context, groupID string) ([]*domain.Item, error) {
	var out []*domain.Item
	pageToken := ""
	for {
		rows, next, err := s.items.List(ctx, "", "", groupID, 100, pageToken)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
		if next == "" {
			break
		}
		pageToken = next
	}
	return out, nil
}

func (s *GroupDeletionService) drainExternalIDs(ctx context.Context, groupID string) ([]*domain.ExternalID, error) {
	var out []*domain.ExternalID
	pageToken := ""
	for {
		rows, next, err := s.externalIDs.List(ctx, domain.EntityTypeGroup, groupID, 100, pageToken)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
		if next == "" {
			break
		}
		pageToken = next
	}
	return out, nil
}

func (s *GroupDeletionService) drainImages(ctx context.Context, groupID string) ([]*domain.Image, error) {
	var out []*domain.Image
	pageToken := ""
	for {
		rows, next, err := s.images.List(ctx, string(domain.EntityTypeGroup), groupID, 100, pageToken)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
		if next == "" {
			break
		}
		pageToken = next
	}
	return out, nil
}

func (s *GroupDeletionService) drainTagAssignments(ctx context.Context, groupID string) ([]*domain.TagAssignment, error) {
	var out []*domain.TagAssignment
	pageToken := ""
	for {
		rows, next, err := s.tagAssignments.List(ctx, "", domain.EntityTypeGroup, groupID, 100, pageToken)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
		if next == "" {
			break
		}
		pageToken = next
	}
	return out, nil
}
