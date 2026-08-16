package service

import (
	"context"
	"purser/internal/domain"
	"purser/internal/domain/music"
	"purser/internal/ports"
)

// LibraryEntryDeletionService is LibraryEntry's composing deletion service
// — the explicit multi-port exception
// docs/adr/0015-deletion-impact-and-composing-services.md carves out.
// internal/service/library_entry.go stays single-port; this is genuinely
// the exception, not a reason to weaken that rule generally.
//
// LibraryEntry is the one kernel entity where "Unlink" doesn't have a
// clean meaning for every referrer: a child LibraryEntry's ParentID is
// optional, so it detaches cleanly (same as Group/Item's other optional-FK
// cases elsewhere in this codebase), but Group.LibraryEntryID and
// Item.LibraryEntryID are both required — they can't be blanked without
// producing an invalid row. So unlike every other composing deletion
// service in this codebase, Delete here actually branches on cascade:
//
//   - cascade=false (Unlink): child LibraryEntries are detached
//     (ParentID cleared, left otherwise intact); if any Group or Item
//     still references this LibraryEntry, Delete fails with
//     ports.ErrDeletionBlocked instead of silently orphaning them —
//     GetDeletionImpact marks those two rows Blocking so a caller can
//     show that before ever attempting Delete.
//   - cascade=true: every child LibraryEntry is recursively
//     cascade-deleted (not just detached — Cascade means "delete the
//     whole subtree"), every Group is cascade-deleted via
//     GroupDeletionService (which itself only detaches Items, never
//     deletes them), and every Item under this LibraryEntry — including
//     ones just detached from a cascade-deleted Group — is deleted via
//     ItemDeletionService.
//
// EntryPerson, ExternalID, Image, and TagAssignment are ordinary
// attachment rows and are always unlinked regardless of cascade, exactly
// like every other composing deletion service.
//
// music.Release.LibraryEntryID is denormalized specifically so an artist
// delete can reach it (docs/adr/0021-music-domain-model.md's "Ripple
// effects" section). Actual Release cleanup falls out of the existing
// cascade into groupDeletion above once it recurses into every Group under
// this LibraryEntry — musicReleases is only used here for an accurate
// GetDeletionImpact count, never for deletion.
type LibraryEntryDeletionService struct {
	libraryEntries ports.LibraryEntryRepository
	groups         ports.GroupRepository
	items          ports.ItemRepository
	entryPeople    ports.EntryPersonRepository
	externalIDs    ports.ExternalIDRepository
	images         ports.ImageRepository
	tagAssignments ports.TagAssignmentRepository
	musicReleases  ports.MusicReleaseRepository
	groupDeletion  *GroupDeletionService
	itemDeletion   *ItemDeletionService
}

// NewLibraryEntryDeletionService constructs a LibraryEntryDeletionService
// backed by the given ports and composing deletion services.
func NewLibraryEntryDeletionService(
	libraryEntries ports.LibraryEntryRepository,
	groups ports.GroupRepository,
	items ports.ItemRepository,
	entryPeople ports.EntryPersonRepository,
	externalIDs ports.ExternalIDRepository,
	images ports.ImageRepository,
	tagAssignments ports.TagAssignmentRepository,
	musicReleases ports.MusicReleaseRepository,
	groupDeletion *GroupDeletionService,
	itemDeletion *ItemDeletionService,
) *LibraryEntryDeletionService {
	return &LibraryEntryDeletionService{
		libraryEntries: libraryEntries,
		groups:         groups,
		items:          items,
		entryPeople:    entryPeople,
		externalIDs:    externalIDs,
		images:         images,
		tagAssignments: tagAssignments,
		musicReleases:  musicReleases,
		groupDeletion:  groupDeletion,
		itemDeletion:   itemDeletion,
	}
}

// GetDeletionImpact returns what references the LibraryEntry identified by
// id, or ports.ErrNotFound if it doesn't exist. The "group" and "item"
// rows are marked Blocking: a non-zero count there means Delete fails
// without cascade=true.
func (s *LibraryEntryDeletionService) GetDeletionImpact(ctx context.Context, id string) (*domain.DeletionImpact, error) {
	if _, err := s.libraryEntries.Get(ctx, id); err != nil {
		return nil, err
	}

	children, err := s.drainChildren(ctx, id)
	if err != nil {
		return nil, err
	}
	groups, err := s.drainGroups(ctx, id)
	if err != nil {
		return nil, err
	}
	items, err := s.drainItems(ctx, id)
	if err != nil {
		return nil, err
	}
	entryPeople, err := s.drainEntryPeople(ctx, id)
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
	musicReleases, err := s.drainMusicReleases(ctx, id)
	if err != nil {
		return nil, err
	}

	return &domain.DeletionImpact{
		Impacts: []domain.DeletionImpactRow{
			{Kind: "library_entry_child", Label: "Child Library Entries (e.g. Studios under this Network)", Count: len(children)},
			{Kind: "group", Label: "Groups", Count: len(groups), Blocking: true},
			{Kind: "item", Label: "Items", Count: len(items), Blocking: true},
			{Kind: "entry_person", Label: "Credits", Count: len(entryPeople)},
			{Kind: "external_id", Label: "External IDs", Count: len(externalIDs)},
			{Kind: "image", Label: "Images", Count: len(images)},
			{Kind: "tag_assignment", Label: "Tags", Count: len(tagAssignments)},
			{Kind: "music_release", Label: "Releases", Count: len(musicReleases)},
		},
	}, nil
}

// Delete removes the LibraryEntry identified by id. See the type doc
// comment for the cascade=false (Unlink, blocks on Groups/Items) versus
// cascade=true (recursively deletes everything) behavior. Returns
// ports.ErrNotFound if the LibraryEntry doesn't exist, or
// ports.ErrDeletionBlocked if cascade=false and Groups/Items exist.
func (s *LibraryEntryDeletionService) Delete(ctx context.Context, id string, cascade bool) error {
	if _, err := s.libraryEntries.Get(ctx, id); err != nil {
		return err
	}

	if cascade {
		if err := s.cascadeDeleteDescendants(ctx, id); err != nil {
			return err
		}
	} else if err := s.unlinkDescendants(ctx, id); err != nil {
		return err
	}

	if err := s.unlinkAttachments(ctx, id); err != nil {
		return err
	}

	return s.libraryEntries.Delete(ctx, id)
}

// unlinkDescendants implements cascade=false: detach child LibraryEntries,
// but block entirely if any Group or Item still references this entry.
func (s *LibraryEntryDeletionService) unlinkDescendants(ctx context.Context, id string) error {
	groups, err := s.drainGroups(ctx, id)
	if err != nil {
		return err
	}
	items, err := s.drainItems(ctx, id)
	if err != nil {
		return err
	}
	if len(groups) > 0 || len(items) > 0 {
		return ports.ErrDeletionBlocked
	}
	return s.detachChildren(ctx, id)
}

// cascadeDeleteDescendants implements cascade=true: recursively delete
// every child LibraryEntry, Group, and Item.
func (s *LibraryEntryDeletionService) cascadeDeleteDescendants(ctx context.Context, id string) error {
	children, err := s.drainChildren(ctx, id)
	if err != nil {
		return err
	}
	for _, c := range children {
		if err := s.Delete(ctx, c.ID, true); err != nil {
			return err
		}
	}

	groups, err := s.drainGroups(ctx, id)
	if err != nil {
		return err
	}
	for _, g := range groups {
		if err := s.groupDeletion.Delete(ctx, g.ID, true); err != nil {
			return err
		}
	}

	items, err := s.drainItems(ctx, id)
	if err != nil {
		return err
	}
	for _, i := range items {
		if err := s.itemDeletion.Delete(ctx, i.ID, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *LibraryEntryDeletionService) detachChildren(ctx context.Context, id string) error {
	children, err := s.drainChildren(ctx, id)
	if err != nil {
		return err
	}
	for _, c := range children {
		c.ParentID = ""
		if err := s.libraryEntries.Update(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func (s *LibraryEntryDeletionService) unlinkAttachments(ctx context.Context, id string) error {
	if err := s.unlinkEntryPeople(ctx, id); err != nil {
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

func (s *LibraryEntryDeletionService) unlinkEntryPeople(ctx context.Context, id string) error {
	rows, err := s.drainEntryPeople(ctx, id)
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

func (s *LibraryEntryDeletionService) unlinkExternalIDs(ctx context.Context, id string) error {
	rows, err := s.drainExternalIDs(ctx, id)
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

func (s *LibraryEntryDeletionService) unlinkImages(ctx context.Context, id string) error {
	rows, err := s.drainImages(ctx, id)
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

func (s *LibraryEntryDeletionService) unlinkTagAssignments(ctx context.Context, id string) error {
	rows, err := s.drainTagAssignments(ctx, id)
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

func (s *LibraryEntryDeletionService) drainChildren(ctx context.Context, parentID string) ([]*domain.LibraryEntry, error) {
	var out []*domain.LibraryEntry
	pageToken := ""
	for {
		rows, next, err := s.libraryEntries.List(ctx, "", parentID, 100, pageToken)
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

func (s *LibraryEntryDeletionService) drainGroups(ctx context.Context, libraryEntryID string) ([]*domain.Group, error) {
	var out []*domain.Group
	pageToken := ""
	for {
		rows, next, err := s.groups.List(ctx, libraryEntryID, 100, pageToken)
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

func (s *LibraryEntryDeletionService) drainItems(ctx context.Context, libraryEntryID string) ([]*domain.Item, error) {
	var out []*domain.Item
	pageToken := ""
	for {
		rows, next, err := s.items.List(ctx, libraryEntryID, "", "", "", 100, pageToken)
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

func (s *LibraryEntryDeletionService) drainEntryPeople(ctx context.Context, libraryEntryID string) ([]*domain.EntryPerson, error) {
	var out []*domain.EntryPerson
	pageToken := ""
	for {
		rows, next, err := s.entryPeople.List(ctx, libraryEntryID, "", 100, pageToken)
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

func (s *LibraryEntryDeletionService) drainExternalIDs(ctx context.Context, libraryEntryID string) ([]*domain.ExternalID, error) {
	var out []*domain.ExternalID
	pageToken := ""
	for {
		rows, next, err := s.externalIDs.List(ctx, domain.EntityTypeLibraryEntry, libraryEntryID, 100, pageToken)
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

func (s *LibraryEntryDeletionService) drainImages(ctx context.Context, libraryEntryID string) ([]*domain.Image, error) {
	var out []*domain.Image
	pageToken := ""
	for {
		rows, next, err := s.images.List(ctx, string(domain.EntityTypeLibraryEntry), libraryEntryID, 100, pageToken)
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

func (s *LibraryEntryDeletionService) drainTagAssignments(ctx context.Context, libraryEntryID string) ([]*domain.TagAssignment, error) {
	var out []*domain.TagAssignment
	pageToken := ""
	for {
		rows, next, err := s.tagAssignments.List(ctx, "", domain.EntityTypeLibraryEntry, libraryEntryID, 100, pageToken)
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

func (s *LibraryEntryDeletionService) drainMusicReleases(ctx context.Context, libraryEntryID string) ([]*music.Release, error) {
	var out []*music.Release
	pageToken := ""
	for {
		rows, next, err := s.musicReleases.ListByEntry(ctx, libraryEntryID, 100, pageToken)
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
