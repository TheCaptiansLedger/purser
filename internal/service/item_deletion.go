package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// ItemDeletionService is Item's composing deletion service — the explicit
// multi-port exception docs/adr/0015-deletion-impact-and-composing-services.md
// carves out. internal/service/item.go stays single-port; this is
// genuinely the exception, not a reason to weaken that rule generally.
//
// Every one of Item's referrers (ItemPerson, MediaFile, ExternalID, Image,
// TagAssignment) is a pure attachment row with no independent value once
// its Item is gone — Item never blocks a delete. cascade is accepted for
// API-shape consistency across every composing deletion service but has
// no effect here.
type ItemDeletionService struct {
	items          ports.ItemRepository
	itemPeople     ports.ItemPersonRepository
	mediaFiles     ports.MediaFileRepository
	externalIDs    ports.ExternalIDRepository
	images         ports.ImageRepository
	tagAssignments ports.TagAssignmentRepository
}

// NewItemDeletionService constructs an ItemDeletionService backed by the
// given ports.
func NewItemDeletionService(
	items ports.ItemRepository,
	itemPeople ports.ItemPersonRepository,
	mediaFiles ports.MediaFileRepository,
	externalIDs ports.ExternalIDRepository,
	images ports.ImageRepository,
	tagAssignments ports.TagAssignmentRepository,
) *ItemDeletionService {
	return &ItemDeletionService{
		items:          items,
		itemPeople:     itemPeople,
		mediaFiles:     mediaFiles,
		externalIDs:    externalIDs,
		images:         images,
		tagAssignments: tagAssignments,
	}
}

// GetDeletionImpact returns what references the Item identified by id, or
// ports.ErrNotFound if the Item doesn't exist.
func (s *ItemDeletionService) GetDeletionImpact(ctx context.Context, id string) (*domain.DeletionImpact, error) {
	if _, err := s.items.Get(ctx, id); err != nil {
		return nil, err
	}

	itemPeople, err := s.drainItemPeople(ctx, id)
	if err != nil {
		return nil, err
	}
	mediaFiles, err := s.drainMediaFiles(ctx, id)
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
			{Kind: "item_person", Label: "Credits", Count: len(itemPeople)},
			{Kind: "media_file", Label: "Media Files", Count: len(mediaFiles)},
			{Kind: "external_id", Label: "External IDs", Count: len(externalIDs)},
			{Kind: "image", Label: "Images", Count: len(images)},
			{Kind: "tag_assignment", Label: "Tags", Count: len(tagAssignments)},
		},
	}, nil
}

// Delete removes the Item identified by id, first unlinking every
// referrer row. Returns ports.ErrNotFound if the Item doesn't exist.
func (s *ItemDeletionService) Delete(ctx context.Context, id string, _ bool) error {
	if _, err := s.items.Get(ctx, id); err != nil {
		return err
	}
	if err := s.unlinkAttachments(ctx, id); err != nil {
		return err
	}
	return s.items.Delete(ctx, id)
}

// DeleteBatch removes every Item in ids, all-or-nothing — see
// docs/adr/0016-bulk-operations.md. Every ID must exist and have its
// attachment rows unlinked before any Item row is removed; the final row
// removal itself is one atomic ports.ItemRepository.DeleteBatch call, not a
// loop of single-row deletes. cascade is accepted for API-shape
// consistency but unused: Item never blocks a delete.
func (s *ItemDeletionService) DeleteBatch(ctx context.Context, ids []string, _ bool) error {
	for _, id := range ids {
		if _, err := s.items.Get(ctx, id); err != nil {
			return err
		}
	}
	for _, id := range ids {
		if err := s.unlinkAttachments(ctx, id); err != nil {
			return err
		}
	}
	return s.items.DeleteBatch(ctx, ids)
}

func (s *ItemDeletionService) unlinkAttachments(ctx context.Context, id string) error {
	if err := s.unlinkItemPeople(ctx, id); err != nil {
		return err
	}
	if err := s.unlinkMediaFiles(ctx, id); err != nil {
		return err
	}
	if err := s.unlinkExternalIDs(ctx, id); err != nil {
		return err
	}
	if err := s.unlinkImages(ctx, id); err != nil {
		return err
	}
	return s.unlinkTagAssignments(ctx, id)
}

func (s *ItemDeletionService) unlinkItemPeople(ctx context.Context, itemID string) error {
	rows, err := s.drainItemPeople(ctx, itemID)
	if err != nil {
		return err
	}
	for _, ip := range rows {
		if err := s.itemPeople.Delete(ctx, ip.ItemID, ip.PersonID, ip.Role); err != nil {
			return err
		}
	}
	return nil
}

func (s *ItemDeletionService) unlinkMediaFiles(ctx context.Context, itemID string) error {
	rows, err := s.drainMediaFiles(ctx, itemID)
	if err != nil {
		return err
	}
	for _, m := range rows {
		if err := s.mediaFiles.Delete(ctx, m.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *ItemDeletionService) unlinkExternalIDs(ctx context.Context, itemID string) error {
	rows, err := s.drainExternalIDs(ctx, itemID)
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

func (s *ItemDeletionService) unlinkImages(ctx context.Context, itemID string) error {
	rows, err := s.drainImages(ctx, itemID)
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

func (s *ItemDeletionService) unlinkTagAssignments(ctx context.Context, itemID string) error {
	rows, err := s.drainTagAssignments(ctx, itemID)
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

func (s *ItemDeletionService) drainItemPeople(ctx context.Context, itemID string) ([]*domain.ItemPerson, error) {
	var out []*domain.ItemPerson
	pageToken := ""
	for {
		rows, next, err := s.itemPeople.List(ctx, itemID, "", 100, pageToken)
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

func (s *ItemDeletionService) drainMediaFiles(ctx context.Context, itemID string) ([]*domain.MediaFile, error) {
	var out []*domain.MediaFile
	pageToken := ""
	for {
		rows, next, err := s.mediaFiles.List(ctx, itemID, 100, pageToken)
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

func (s *ItemDeletionService) drainExternalIDs(ctx context.Context, itemID string) ([]*domain.ExternalID, error) {
	var out []*domain.ExternalID
	pageToken := ""
	for {
		rows, next, err := s.externalIDs.List(ctx, domain.EntityTypeItem, itemID, 100, pageToken)
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

func (s *ItemDeletionService) drainImages(ctx context.Context, itemID string) ([]*domain.Image, error) {
	var out []*domain.Image
	pageToken := ""
	for {
		rows, next, err := s.images.List(ctx, string(domain.EntityTypeItem), itemID, 100, pageToken)
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

func (s *ItemDeletionService) drainTagAssignments(ctx context.Context, itemID string) ([]*domain.TagAssignment, error) {
	var out []*domain.TagAssignment
	pageToken := ""
	for {
		rows, next, err := s.tagAssignments.List(ctx, "", domain.EntityTypeItem, itemID, 100, pageToken)
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
