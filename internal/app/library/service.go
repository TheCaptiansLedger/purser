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
}

func New(
	entries ports.LibraryEntryRepository,
	groups ports.GroupRepository,
	items ports.ItemRepository,
) *Service {
	return &Service{entries: entries, groups: groups, items: items}
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

func (s *Service) GetEntry(ctx context.Context, id string) (*domain.LibraryEntry, error) {
	e, err := s.entries.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	return e, err
}

func (s *Service) ListEntries(ctx context.Context, f ports.LibraryFilter) ([]*domain.LibraryEntry, int, error) {
	return s.entries.List(ctx, f)
}

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

func (s *Service) DeleteEntry(ctx context.Context, id string) error {
	if _, err := s.GetEntry(ctx, id); err != nil {
		return err
	}
	return s.entries.Delete(ctx, id)
}

// ── Groups ────────────────────────────────────────────────────────────────────

func (s *Service) CreateGroup(ctx context.Context, g *domain.Group) error {
	if g.LibraryEntryID == "" {
		return errs.Validation("libraryEntryId is required")
	}
	if g.MonitorMode == "" {
		g.MonitorMode = domain.MonitorAll
	}
	return s.groups.Save(ctx, g)
}

func (s *Service) GetGroup(ctx context.Context, id string) (*domain.Group, error) {
	g, err := s.groups.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	return g, err
}

func (s *Service) ListGroups(ctx context.Context, f ports.GroupFilter) ([]*domain.Group, error) {
	return s.groups.List(ctx, f)
}

func (s *Service) SaveGroup(ctx context.Context, g *domain.Group) error {
	return s.groups.Save(ctx, g)
}

func (s *Service) DeleteGroup(ctx context.Context, id string) error {
	if _, err := s.GetGroup(ctx, id); err != nil {
		return err
	}
	return s.groups.Delete(ctx, id)
}

// ── Items ─────────────────────────────────────────────────────────────────────

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

func (s *Service) GetItem(ctx context.Context, id string) (*domain.Item, error) {
	item, err := s.items.Get(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.ErrNotFound
	}
	return item, err
}

func (s *Service) ListItems(ctx context.Context, f ports.ItemFilter) ([]*domain.Item, int, error) {
	return s.items.List(ctx, f)
}

func (s *Service) SaveItem(ctx context.Context, item *domain.Item) error {
	return s.items.Save(ctx, item)
}

func (s *Service) DeleteItem(ctx context.Context, id string) error {
	if _, err := s.GetItem(ctx, id); err != nil {
		return err
	}
	return s.items.Delete(ctx, id)
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
