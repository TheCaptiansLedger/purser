package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// TagDeletionService is Tag's composing deletion service — the explicit
// multi-port exception docs/adr/0015-deletion-impact-and-composing-services.md
// carves out. internal/service/tag.go stays single-port; this is genuinely
// the exception, not a reason to weaken that rule generally.
//
// Tag's only referrer is TagAssignment, a pure join row with no required
// (non-nullable) foreign key pointing the other way — so Tag never blocks
// a delete. cascade is accepted for API-shape consistency across every
// composing deletion service but has no effect here.
type TagDeletionService struct {
	tags           ports.TagRepository
	tagAssignments ports.TagAssignmentRepository
}

// NewTagDeletionService constructs a TagDeletionService backed by the
// given ports.
func NewTagDeletionService(tags ports.TagRepository, tagAssignments ports.TagAssignmentRepository) *TagDeletionService {
	return &TagDeletionService{tags: tags, tagAssignments: tagAssignments}
}

// GetDeletionImpact returns what references the Tag identified by id, or
// ports.ErrNotFound if the Tag doesn't exist.
func (s *TagDeletionService) GetDeletionImpact(ctx context.Context, id string) (*domain.DeletionImpact, error) {
	if _, err := s.tags.Get(ctx, id); err != nil {
		return nil, err
	}

	assignments, err := s.drainAssignments(ctx, id)
	if err != nil {
		return nil, err
	}

	return &domain.DeletionImpact{
		Impacts: []domain.DeletionImpactRow{
			{Kind: "tag_assignment", Label: "Tag Assignments", Count: len(assignments)},
		},
	}, nil
}

// Delete removes the Tag identified by id, first unlinking every
// TagAssignment row that references it. Returns ports.ErrNotFound if the
// Tag doesn't exist.
func (s *TagDeletionService) Delete(ctx context.Context, id string, _ bool) error {
	if _, err := s.tags.Get(ctx, id); err != nil {
		return err
	}
	if err := s.unlinkAssignments(ctx, id); err != nil {
		return err
	}
	return s.tags.Delete(ctx, id)
}

// DeleteBatch removes every Tag in ids, all-or-nothing — see
// docs/adr/0016-bulk-operations.md. Every ID must exist and have its
// TagAssignments unlinked before any Tag row is removed; the final row
// removal itself is one atomic ports.TagRepository.DeleteBatch call, not a
// loop of single-row deletes. cascade is accepted for API-shape
// consistency but unused: Tag never blocks a delete.
func (s *TagDeletionService) DeleteBatch(ctx context.Context, ids []string, _ bool) error {
	for _, id := range ids {
		if _, err := s.tags.Get(ctx, id); err != nil {
			return err
		}
	}
	for _, id := range ids {
		if err := s.unlinkAssignments(ctx, id); err != nil {
			return err
		}
	}
	return s.tags.DeleteBatch(ctx, ids)
}

func (s *TagDeletionService) unlinkAssignments(ctx context.Context, tagID string) error {
	assignments, err := s.drainAssignments(ctx, tagID)
	if err != nil {
		return err
	}
	for _, ta := range assignments {
		if err := s.tagAssignments.Delete(ctx, ta.TagID, ta.EntityType, ta.EntityID); err != nil {
			return err
		}
	}
	return nil
}

func (s *TagDeletionService) drainAssignments(ctx context.Context, tagID string) ([]*domain.TagAssignment, error) {
	var out []*domain.TagAssignment
	pageToken := ""
	for {
		rows, next, err := s.tagAssignments.List(ctx, tagID, "", "", 100, pageToken)
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
