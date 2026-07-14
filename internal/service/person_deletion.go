package service

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
)

// PersonDeletionService is Person's composing deletion service — the
// explicit multi-port exception
// docs/adr/0015-deletion-impact-and-composing-services.md carves out.
// internal/service/person.go stays single-port; this is genuinely the
// exception, not a reason to weaken that rule generally.
//
// Every one of Person's referrers (EntryPerson, ItemPerson, ExternalID,
// Image, TagAssignment, afterdark.PerformerProfile) is a pure attachment
// row with no independent value once its Person is gone — Person never
// blocks a delete. cascade is accepted for API-shape consistency across
// every composing deletion service but has no effect here.
//
// Depending on ports.PerformerProfileRepository — an AfterDark-module
// port — from a kernel composing service is a deliberate reach-across,
// not an oversight: ADR 0015's own Context section already names
// afterdark.PerformerProfile as one of the rows Person's deletion-impact
// accounting must cover, and composing services are already the
// sanctioned exception to normal port-count limits. Treating "reach into
// a module's port for cleanup" as within that same exception (rather than
// inventing a plugin-registration mechanism for one module) is the
// reading this type implements.
type PersonDeletionService struct {
	people         ports.PersonRepository
	entryPeople    ports.EntryPersonRepository
	itemPeople     ports.ItemPersonRepository
	externalIDs    ports.ExternalIDRepository
	images         ports.ImageRepository
	tagAssignments ports.TagAssignmentRepository
	profiles       ports.PerformerProfileRepository
}

// NewPersonDeletionService constructs a PersonDeletionService backed by
// the given ports.
func NewPersonDeletionService(
	people ports.PersonRepository,
	entryPeople ports.EntryPersonRepository,
	itemPeople ports.ItemPersonRepository,
	externalIDs ports.ExternalIDRepository,
	images ports.ImageRepository,
	tagAssignments ports.TagAssignmentRepository,
	profiles ports.PerformerProfileRepository,
) *PersonDeletionService {
	return &PersonDeletionService{
		people:         people,
		entryPeople:    entryPeople,
		itemPeople:     itemPeople,
		externalIDs:    externalIDs,
		images:         images,
		tagAssignments: tagAssignments,
		profiles:       profiles,
	}
}

// GetDeletionImpact returns what references the Person identified by id,
// or ports.ErrNotFound if the Person doesn't exist.
func (s *PersonDeletionService) GetDeletionImpact(ctx context.Context, id string) (*domain.DeletionImpact, error) {
	if _, err := s.people.Get(ctx, id); err != nil {
		return nil, err
	}

	entryPeople, err := s.drainEntryPeople(ctx, id)
	if err != nil {
		return nil, err
	}
	itemPeople, err := s.drainItemPeople(ctx, id)
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
	hasProfile, err := s.hasPerformerProfile(ctx, id)
	if err != nil {
		return nil, err
	}
	profileCount := 0
	if hasProfile {
		profileCount = 1
	}

	return &domain.DeletionImpact{
		Impacts: []domain.DeletionImpactRow{
			{Kind: "entry_person", Label: "Credits (Library Entries)", Count: len(entryPeople)},
			{Kind: "item_person", Label: "Credits (Items)", Count: len(itemPeople)},
			{Kind: "external_id", Label: "External IDs", Count: len(externalIDs)},
			{Kind: "image", Label: "Images", Count: len(images)},
			{Kind: "tag_assignment", Label: "Tags", Count: len(tagAssignments)},
			{Kind: "performer_profile", Label: "AfterDark Performer Profile", Count: profileCount},
		},
	}, nil
}

// Delete removes the Person identified by id, first unlinking every
// referrer row. Returns ports.ErrNotFound if the Person doesn't exist.
func (s *PersonDeletionService) Delete(ctx context.Context, id string, _ bool) error {
	if _, err := s.people.Get(ctx, id); err != nil {
		return err
	}
	if err := s.unlinkEntryPeople(ctx, id); err != nil {
		return err
	}
	if err := s.unlinkItemPeople(ctx, id); err != nil {
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
	if err := s.unlinkPerformerProfile(ctx, id); err != nil {
		return err
	}
	return s.people.Delete(ctx, id)
}

func (s *PersonDeletionService) unlinkEntryPeople(ctx context.Context, personID string) error {
	rows, err := s.drainEntryPeople(ctx, personID)
	if err != nil {
		return err
	}
	for _, ep := range rows {
		if err := s.entryPeople.Delete(ctx, ep.LibraryEntryID, ep.PersonID, ep.Role); err != nil {
			return err
		}
	}
	return nil
}

func (s *PersonDeletionService) unlinkItemPeople(ctx context.Context, personID string) error {
	rows, err := s.drainItemPeople(ctx, personID)
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

func (s *PersonDeletionService) unlinkExternalIDs(ctx context.Context, personID string) error {
	rows, err := s.drainExternalIDs(ctx, personID)
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

func (s *PersonDeletionService) unlinkImages(ctx context.Context, personID string) error {
	rows, err := s.drainImages(ctx, personID)
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

func (s *PersonDeletionService) unlinkTagAssignments(ctx context.Context, personID string) error {
	rows, err := s.drainTagAssignments(ctx, personID)
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

func (s *PersonDeletionService) unlinkPerformerProfile(ctx context.Context, personID string) error {
	has, err := s.hasPerformerProfile(ctx, personID)
	if err != nil {
		return err
	}
	if !has {
		return nil
	}
	return s.profiles.Delete(ctx, personID)
}

func (s *PersonDeletionService) hasPerformerProfile(ctx context.Context, personID string) (bool, error) {
	_, err := s.profiles.Get(ctx, personID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ports.ErrNotFound) {
		return false, nil
	}
	return false, err
}

func (s *PersonDeletionService) drainEntryPeople(ctx context.Context, personID string) ([]*domain.EntryPerson, error) {
	var out []*domain.EntryPerson
	pageToken := ""
	for {
		rows, next, err := s.entryPeople.List(ctx, "", personID, 100, pageToken)
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

func (s *PersonDeletionService) drainItemPeople(ctx context.Context, personID string) ([]*domain.ItemPerson, error) {
	var out []*domain.ItemPerson
	pageToken := ""
	for {
		rows, next, err := s.itemPeople.List(ctx, "", personID, 100, pageToken)
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

func (s *PersonDeletionService) drainExternalIDs(ctx context.Context, personID string) ([]*domain.ExternalID, error) {
	var out []*domain.ExternalID
	pageToken := ""
	for {
		rows, next, err := s.externalIDs.List(ctx, domain.EntityTypePerson, personID, 100, pageToken)
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

func (s *PersonDeletionService) drainImages(ctx context.Context, personID string) ([]*domain.Image, error) {
	var out []*domain.Image
	pageToken := ""
	for {
		rows, next, err := s.images.List(ctx, string(domain.EntityTypePerson), personID, 100, pageToken)
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

func (s *PersonDeletionService) drainTagAssignments(ctx context.Context, personID string) ([]*domain.TagAssignment, error) {
	var out []*domain.TagAssignment
	pageToken := ""
	for {
		rows, next, err := s.tagAssignments.List(ctx, "", domain.EntityTypePerson, personID, 100, pageToken)
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
