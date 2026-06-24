package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
)

// Service handles the library hierarchy: entries, groups, and items.
// It enforces invariants and business rules that the repository layer does not.
type Service struct {
	entries ports.LibraryEntryRepository
	groups  ports.GroupRepository
	items   ports.ItemRepository
	persons ports.PersonRepository
	tags    ports.TagRepository
}

// New constructs a library Service wired to the given repositories.
func New(
	entries ports.LibraryEntryRepository,
	groups ports.GroupRepository,
	items ports.ItemRepository,
	persons ports.PersonRepository,
	tags ports.TagRepository,
) *Service {
	return &Service{entries: entries, groups: groups, items: items, persons: persons, tags: tags}
}

// ── Library entries ───────────────────────────────────────────────────────────

// CreateEntry validates e, applies defaults, persists it, and for kind=movie
// auto-creates the corresponding leaf item so the entry is immediately trackable.
func (s *Service) CreateEntry(ctx context.Context, e *domain.LibraryEntry) error {
	if err := validateEntry(e); err != nil {
		return err
	}
	applyEntryDefaults(e)
	if err := s.entries.Save(ctx, e); err != nil {
		return fmt.Errorf("save entry: %w", err)
	}
	if e.Kind == domain.KindMovie {
		item := &domain.Item{
			ContentType:    e.ContentType,
			LibraryEntryID: e.ID,
			Title:          e.Name,
			Overview:       e.Overview,
			Monitored:      e.Monitored,
			Status:         domain.StatusWanted,
			ExternalIDs:    e.ExternalIDs,
		}
		if err := s.items.Save(ctx, item); err != nil {
			return fmt.Errorf("auto-create movie item: %w", err)
		}
	}
	return nil
}

// GetEntry returns the library entry with the given ID.
func (s *Service) GetEntry(ctx context.Context, id string) (*domain.LibraryEntry, error) {
	e, err := s.entries.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	return e, err
}

// ListEntries returns a filtered, paginated list of library entries.
func (s *Service) ListEntries(ctx context.Context, f ports.LibraryFilter) ([]*domain.LibraryEntry, int, error) {
	return s.entries.List(ctx, f)
}

// ListChildren returns the direct children of the given library entry.
func (s *Service) ListChildren(ctx context.Context, parentID string) ([]*domain.LibraryEntry, int, error) {
	return s.entries.List(ctx, ports.LibraryFilter{ParentID: parentID, Limit: 200})
}

// SaveEntry validates and persists an existing entry. Used for updates.
func (s *Service) SaveEntry(ctx context.Context, e *domain.LibraryEntry) error {
	if err := validateEntry(e); err != nil {
		return err
	}
	return s.entries.Save(ctx, e)
}

// DeleteEntry removes a library entry by ID, returning an error if not found.
func (s *Service) DeleteEntry(ctx context.Context, id string) error {
	if _, err := s.GetEntry(ctx, id); err != nil {
		return err
	}
	return s.entries.Delete(ctx, id)
}

// ── Groups ────────────────────────────────────────────────────────────────────

// CreateGroup validates and persists a new group under a library entry.
func (s *Service) CreateGroup(ctx context.Context, g *domain.Group) error {
	if g.LibraryEntryID == "" {
		return errs.Validation("libraryEntryId is required")
	}
	if g.MonitorMode == "" {
		g.MonitorMode = domain.MonitorAll
	}
	return s.groups.Save(ctx, g)
}

// GetGroup returns the group with the given ID.
func (s *Service) GetGroup(ctx context.Context, id string) (*domain.Group, error) {
	g, err := s.groups.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	return g, err
}

// ListGroups returns groups matching the given filter.
func (s *Service) ListGroups(ctx context.Context, f ports.GroupFilter) ([]*domain.Group, error) {
	return s.groups.List(ctx, f)
}

// SaveGroup persists changes to an existing group.
func (s *Service) SaveGroup(ctx context.Context, g *domain.Group) error {
	return s.groups.Save(ctx, g)
}

// DeleteGroup removes a group by ID, returning an error if not found.
func (s *Service) DeleteGroup(ctx context.Context, id string) error {
	if _, err := s.GetGroup(ctx, id); err != nil {
		return err
	}
	return s.groups.Delete(ctx, id)
}

// AddGroupTag links an existing tag to a group.
func (s *Service) AddGroupTag(ctx context.Context, groupID, tagID string) error {
	if _, err := s.GetGroup(ctx, groupID); err != nil {
		return err
	}
	return s.tags.AddGroupTag(ctx, groupID, tagID)
}

// RemoveGroupTag unlinks a tag from a group.
func (s *Service) RemoveGroupTag(ctx context.Context, groupID, tagID string) error {
	return s.tags.RemoveGroupTag(ctx, groupID, tagID)
}

// ── Items ─────────────────────────────────────────────────────────────────────

// CreateItem validates and persists a new leaf item.
func (s *Service) CreateItem(ctx context.Context, item *domain.Item) error {
	if item.LibraryEntryID == "" {
		return errs.Validation("libraryEntryId is required")
	}
	if item.Status == "" {
		item.Status = domain.StatusWanted
	}
	// Inherit content type from parent if not specified.
	if !item.ContentType.Valid() {
		parent, err := s.entries.Get(ctx, item.LibraryEntryID)
		if err == nil {
			item.ContentType = parent.ContentType
		}
	}
	return s.items.Save(ctx, item)
}

// GetItem returns the item with the given ID.
func (s *Service) GetItem(ctx context.Context, id string) (*domain.Item, error) {
	item, err := s.items.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	return item, err
}

// ListItems returns a filtered, paginated list of items.
func (s *Service) ListItems(ctx context.Context, f ports.ItemFilter) ([]*domain.Item, int, error) {
	return s.items.List(ctx, f)
}

// SaveItem persists changes to an existing item.
func (s *Service) SaveItem(ctx context.Context, item *domain.Item) error {
	return s.items.Save(ctx, item)
}

// ErrInvalidStatusForUserUpdate is returned when the requested status cannot
// be set by a user action (as opposed to an automated pipeline transition).
var ErrInvalidStatusForUserUpdate = errors.New("status cannot be set by user action")

// UpdateItemStatus validates and applies a user-initiated status change.
// Only StatusWanted and StatusSkipped may be set via user action.
func (s *Service) UpdateItemStatus(ctx context.Context, id string, newStatus domain.ItemStatus) error {
	if newStatus != domain.StatusWanted && newStatus != domain.StatusSkipped {
		return ErrInvalidStatusForUserUpdate
	}
	item, err := s.GetItem(ctx, id)
	if err != nil {
		return err
	}
	if err := domain.ValidateTransition(item.Status, newStatus); err != nil {
		return err
	}
	item.Status = newStatus
	return s.items.Save(ctx, item)
}

// DeleteItem removes an item by ID, returning an error if not found.
func (s *Service) DeleteItem(ctx context.Context, id string) error {
	if _, err := s.GetItem(ctx, id); err != nil {
		return err
	}
	return s.items.Delete(ctx, id)
}

// ── Entry people (artist members) ────────────────────────────────────────────

// GetEntryPeople returns all member links for the given entry.
func (s *Service) GetEntryPeople(ctx context.Context, entryID string) ([]domain.EntryPerson, error) {
	return s.entries.GetPeople(ctx, entryID)
}

// SaveEntryPerson upserts a single member link for the given entry.
func (s *Service) SaveEntryPerson(ctx context.Context, entryID string, ep domain.EntryPerson) error {
	return s.entries.SavePerson(ctx, entryID, ep)
}

// RemoveEntryPerson removes a single member link for the given entry.
func (s *Service) RemoveEntryPerson(ctx context.Context, entryID, personID, role string) error {
	return s.entries.RemovePerson(ctx, entryID, personID, role)
}

// ImportArtistMembers upserts each person in members and links them to the given artist entry
// under the provided role. The entry must support member relationships.
func (s *Service) ImportArtistMembers(ctx context.Context, entryID string, members []domain.Person, role string) error {
	entry, err := s.GetEntry(ctx, entryID)
	if err != nil {
		return err
	}
	if !entry.Kind.SupportsMemberRelationships() {
		return errs.Validation("ImportArtistMembers: entry must support member relationships")
	}
	for i := range members {
		if err := s.persons.Save(ctx, &members[i]); err != nil {
			return fmt.Errorf("upsert person %q: %w", members[i].Name, err)
		}
		ep := domain.EntryPerson{PersonID: members[i].ID, Role: role}
		if err := s.entries.SavePerson(ctx, entryID, ep); err != nil {
			return fmt.Errorf("link person %q to entry: %w", members[i].ID, err)
		}
	}
	return nil
}

// ── Item people (cast) ────────────────────────────────────────────────────────

// SaveItemPerson upserts a single cast link for the given item.
func (s *Service) SaveItemPerson(ctx context.Context, itemID string, ip domain.ItemPerson) error {
	item, err := s.GetItem(ctx, itemID)
	if err != nil {
		return err
	}
	filtered := make([]domain.ItemPerson, 0, len(item.People))
	for _, p := range item.People {
		if p.PersonID != ip.PersonID || p.Role != ip.Role {
			filtered = append(filtered, p)
		}
	}
	item.People = append(filtered, ip)
	return s.SaveItem(ctx, item)
}

// RemoveItemPerson removes a single cast link for the given item.
func (s *Service) RemoveItemPerson(ctx context.Context, itemID, personID, role string) error {
	item, err := s.GetItem(ctx, itemID)
	if err != nil {
		return err
	}
	filtered := make([]domain.ItemPerson, 0, len(item.People))
	for _, p := range item.People {
		if p.PersonID != personID || string(p.Role) != role {
			filtered = append(filtered, p)
		}
	}
	item.People = filtered
	return s.SaveItem(ctx, item)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func validateEntry(e *domain.LibraryEntry) error {
	if e.Name == "" {
		return errs.Validation("name is required")
	}
	if !e.ContentType.Valid() {
		return errs.Validation(fmt.Sprintf("invalid content type: %q", e.ContentType))
	}
	if !e.Kind.Valid() {
		return errs.Validation(fmt.Sprintf("invalid kind: %q", e.Kind))
	}
	return nil
}

func applyEntryDefaults(e *domain.LibraryEntry) {
	if e.SortName == "" {
		e.SortName = e.Name
	}
	if e.MonitorMode == "" {
		e.MonitorMode = domain.MonitorAll
	}
	if e.Status == "" {
		e.Status = domain.EntryStatusActive
	}
}
