package service

import (
	"context"
	"errors"
	"fmt"
	"purser/internal/domain"
	"purser/internal/domain/afterdark"
	"purser/internal/ports"
	"strconv"
)

// innerListPageSize bounds each internal (non-caller-facing) List call this
// service issues while assembling a full result set to compose/paginate
// over — see the pagination note on AfterDarkBrowseService.
const innerListPageSize = 100

// AfterDarkBrowseService answers cross-entity AfterDark reads that no
// single entity's own port/service can answer about itself — the explicit
// composing-service exception carved out by
// docs/adr/0011-api-design.md ("A handler assembling a composed view... is
// a future, separate, explicitly composing service — never folded into a
// single-entity CRUD service") and
// docs/adr/0015-deletion-impact-and-composing-services.md (the first
// standing instance of that exception). It depends on
// ports.LibraryEntryRepository, ports.ItemRepository,
// ports.ItemPersonRepository, ports.PersonRepository, and
// ports.PerformerProfileRepository together — deliberately never folded
// into LibraryEntryService/ItemService/ItemPersonService/PersonService,
// each of which stays single-port per ADR 0011's "no God service" rule.
//
// This is AfterDark-specific (Kind=network/studio and the PerformerView
// composition are AfterDark concepts, not kernel ones), not a kernel-wide
// service — a future module needing the same shape (e.g. Movies
// "everything from a franchise") gets its own composing service, not a
// shared one, per docs/adr/0001-hexagonal-architecture.md's rule against
// content-type conditionals in shared code.
//
// Pagination note: methods that union results across multiple parents
// (ListScenesInNetwork, ListPerformersForStudio, ListPerformersForNetwork)
// read every matching row into memory before applying this service's own
// offset-encoded page token over the assembled slice — a deliberate,
// documented simplification for AfterDark's bounded fan-out (studios under
// a network, scenes under a studio), not a substitute for real
// storage-backed cursor pagination on a possibly-unbounded list.
// ListPerformers is the exception: it's driven directly by
// PerformerProfileRepository.List's own real cursor, since no fan-out is
// involved.
type AfterDarkBrowseService struct {
	libraryEntries ports.LibraryEntryRepository
	items          ports.ItemRepository
	itemPeople     ports.ItemPersonRepository
	people         ports.PersonRepository
	profiles       ports.PerformerProfileRepository
}

// NewAfterDarkBrowseService constructs an AfterDarkBrowseService backed by
// the given ports.
func NewAfterDarkBrowseService(
	libraryEntries ports.LibraryEntryRepository,
	items ports.ItemRepository,
	itemPeople ports.ItemPersonRepository,
	people ports.PersonRepository,
	profiles ports.PerformerProfileRepository,
) *AfterDarkBrowseService {
	return &AfterDarkBrowseService{
		libraryEntries: libraryEntries,
		items:          items,
		itemPeople:     itemPeople,
		people:         people,
		profiles:       profiles,
	}
}

// ListScenesInNetwork returns every Item(ContentType=adult) belonging to a
// Studio (Kind=studio) whose ParentID is networkID.
func (s *AfterDarkBrowseService) ListScenesInNetwork(ctx context.Context, networkID string, pageSize int, pageToken string) ([]*domain.Item, string, error) {
	studios, err := s.allStudiosUnderNetwork(ctx, networkID)
	if err != nil {
		return nil, "", err
	}

	var scenes []*domain.Item
	for _, studio := range studios {
		items, err := s.allScenesForLibraryEntry(ctx, studio.ID)
		if err != nil {
			return nil, "", err
		}
		scenes = append(scenes, items...)
	}
	return paginateSlice(scenes, pageSize, pageToken)
}

// ListScenesForPerformer returns every Item(ContentType=adult) a performer
// (identified by personID) is credited on, resolving ItemPerson join rows
// back to their Item.
func (s *AfterDarkBrowseService) ListScenesForPerformer(ctx context.Context, personID string, pageSize int, pageToken string) ([]*domain.Item, string, error) {
	rows, err := s.allItemPeopleForPerson(ctx, personID)
	if err != nil {
		return nil, "", err
	}

	seen := map[string]bool{}
	var scenes []*domain.Item
	for _, row := range rows {
		if seen[row.ItemID] {
			continue
		}
		seen[row.ItemID] = true
		item, err := s.items.Get(ctx, row.ItemID)
		if err != nil {
			if errors.Is(err, ports.ErrNotFound) {
				continue
			}
			return nil, "", err
		}
		scenes = append(scenes, item)
	}
	return paginateSlice(scenes, pageSize, pageToken)
}

// ListPerformersForScene returns every PerformerView credited on the Item
// identified by itemID.
func (s *AfterDarkBrowseService) ListPerformersForScene(ctx context.Context, itemID string, pageSize int, pageToken string) ([]*afterdark.PerformerView, string, error) {
	rows, err := s.allItemPeopleForItem(ctx, itemID)
	if err != nil {
		return nil, "", err
	}
	views, err := s.performerViewsForPersonIDs(ctx, personIDsOf(rows))
	if err != nil {
		return nil, "", err
	}
	return paginateSlice(views, pageSize, pageToken)
}

// ListPerformersForStudio returns every PerformerView credited on any
// scene belonging to the LibraryEntry (Kind=studio) identified by
// libraryEntryID.
func (s *AfterDarkBrowseService) ListPerformersForStudio(ctx context.Context, libraryEntryID string, pageSize int, pageToken string) ([]*afterdark.PerformerView, string, error) {
	scenes, err := s.allScenesForLibraryEntry(ctx, libraryEntryID)
	if err != nil {
		return nil, "", err
	}

	var personIDs []string
	for _, scene := range scenes {
		rows, err := s.allItemPeopleForItem(ctx, scene.ID)
		if err != nil {
			return nil, "", err
		}
		personIDs = append(personIDs, personIDsOf(rows)...)
	}

	views, err := s.performerViewsForPersonIDs(ctx, personIDs)
	if err != nil {
		return nil, "", err
	}
	return paginateSlice(views, pageSize, pageToken)
}

// ListPerformersForNetwork returns every PerformerView credited on any
// scene belonging to any Studio under the Network identified by networkID.
func (s *AfterDarkBrowseService) ListPerformersForNetwork(ctx context.Context, networkID string, pageSize int, pageToken string) ([]*afterdark.PerformerView, string, error) {
	studios, err := s.allStudiosUnderNetwork(ctx, networkID)
	if err != nil {
		return nil, "", err
	}

	var personIDs []string
	for _, studio := range studios {
		scenes, err := s.allScenesForLibraryEntry(ctx, studio.ID)
		if err != nil {
			return nil, "", err
		}
		for _, scene := range scenes {
			rows, err := s.allItemPeopleForItem(ctx, scene.ID)
			if err != nil {
				return nil, "", err
			}
			personIDs = append(personIDs, personIDsOf(rows)...)
		}
	}

	views, err := s.performerViewsForPersonIDs(ctx, personIDs)
	if err != nil {
		return nil, "", err
	}
	return paginateSlice(views, pageSize, pageToken)
}

// ListPerformers returns every Person that has a PerformerProfile,
// composed into a PerformerView — the catalog-wide "every performer"
// listing, distinct from PersonService.List (which lists every Person
// regardless of module). Real cursor pagination, driven directly by
// PerformerProfileRepository.List.
func (s *AfterDarkBrowseService) ListPerformers(ctx context.Context, pageSize int, pageToken string) ([]*afterdark.PerformerView, string, error) {
	profiles, next, err := s.profiles.List(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", err
	}

	views := make([]*afterdark.PerformerView, 0, len(profiles))
	for _, profile := range profiles {
		person, err := s.people.Get(ctx, profile.PersonID)
		if err != nil {
			if errors.Is(err, ports.ErrNotFound) {
				continue
			}
			return nil, "", err
		}
		views = append(views, &afterdark.PerformerView{Person: person, Profile: profile})
	}
	return views, next, nil
}

// allStudiosUnderNetwork fully drains LibraryEntryRepository.List(kind=studio,
// parentID=networkID) across every underlying page.
func (s *AfterDarkBrowseService) allStudiosUnderNetwork(ctx context.Context, networkID string) ([]*domain.LibraryEntry, error) {
	var out []*domain.LibraryEntry
	pageToken := ""
	for {
		entries, next, err := s.libraryEntries.List(ctx, domain.KindStudio, networkID, innerListPageSize, pageToken)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
		if next == "" {
			break
		}
		pageToken = next
	}
	return out, nil
}

// allScenesForLibraryEntry fully drains
// ItemRepository.List(libraryEntryID=libraryEntryID, contentType=adult)
// across every underlying page.
func (s *AfterDarkBrowseService) allScenesForLibraryEntry(ctx context.Context, libraryEntryID string) ([]*domain.Item, error) {
	var out []*domain.Item
	pageToken := ""
	for {
		items, next, err := s.items.List(ctx, libraryEntryID, string(domain.ContentTypeAdult), "", "", innerListPageSize, pageToken)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
		if next == "" {
			break
		}
		pageToken = next
	}
	return out, nil
}

// allItemPeopleForItem fully drains
// ItemPersonRepository.List(itemID=itemID) across every underlying page.
func (s *AfterDarkBrowseService) allItemPeopleForItem(ctx context.Context, itemID string) ([]*domain.ItemPerson, error) {
	return s.allItemPeople(ctx, itemID, "")
}

// allItemPeopleForPerson fully drains
// ItemPersonRepository.List(personID=personID) across every underlying
// page.
func (s *AfterDarkBrowseService) allItemPeopleForPerson(ctx context.Context, personID string) ([]*domain.ItemPerson, error) {
	return s.allItemPeople(ctx, "", personID)
}

func (s *AfterDarkBrowseService) allItemPeople(ctx context.Context, itemID, personID string) ([]*domain.ItemPerson, error) {
	var out []*domain.ItemPerson
	pageToken := ""
	for {
		rows, next, err := s.itemPeople.List(ctx, itemID, personID, innerListPageSize, pageToken)
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

// performerViewsForPersonIDs resolves each distinct PersonID to a
// PerformerView (Person + PerformerProfile), skipping any PersonID with no
// PerformerProfile — per PerformerView's doc comment, a credited Person
// with no profile isn't yet a "Performer" for AfterDark's purposes.
// personIDs is deduplicated here, not just by its callers: callers that
// union across multiple scenes (ListPerformersForStudio,
// ListPerformersForNetwork) can otherwise pass the same PersonID more than
// once when a performer is credited on several scenes under the same
// studio/network.
func (s *AfterDarkBrowseService) performerViewsForPersonIDs(ctx context.Context, personIDs []string) ([]*afterdark.PerformerView, error) {
	seen := map[string]bool{}
	views := make([]*afterdark.PerformerView, 0, len(personIDs))
	for _, personID := range personIDs {
		if seen[personID] {
			continue
		}
		seen[personID] = true
		profile, err := s.profiles.Get(ctx, personID)
		if err != nil {
			if errors.Is(err, ports.ErrNotFound) {
				continue
			}
			return nil, err
		}
		person, err := s.people.Get(ctx, personID)
		if err != nil {
			if errors.Is(err, ports.ErrNotFound) {
				continue
			}
			return nil, err
		}
		views = append(views, &afterdark.PerformerView{Person: person, Profile: profile})
	}
	return views, nil
}

// personIDsOf extracts each row's PersonID, deduplicated, preserving
// first-seen order.
func personIDsOf(rows []*domain.ItemPerson) []string {
	seen := map[string]bool{}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if seen[row.PersonID] {
			continue
		}
		seen[row.PersonID] = true
		ids = append(ids, row.PersonID)
	}
	return ids
}

// paginateSlice applies this composing service's own pagination over an
// already-fully-assembled, in-memory slice — see the pagination note on
// AfterDarkBrowseService. The page token is a base-10 offset into items,
// still opaque to callers (they only ever round-trip it verbatim), just
// not a real storage-backed cursor.
func paginateSlice[T any](items []T, pageSize int, pageToken string) ([]T, string, error) {
	offset := 0
	if pageToken != "" {
		o, err := strconv.Atoi(pageToken)
		if err != nil || o < 0 {
			return nil, "", fmt.Errorf("service: invalid page token %q", pageToken)
		}
		offset = o
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if offset >= len(items) {
		return []T{}, "", nil
	}
	end := min(offset+pageSize, len(items))
	next := ""
	if end < len(items) {
		next = strconv.Itoa(end)
	}
	return items[offset:end], next, nil
}
