package metadata_test

import (
	"context"
	"fmt"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
)

// ── Entry repo stub ───────────────────────────────────────────────────────────

type stubEntryRepo struct {
	data map[string]*domain.LibraryEntry
}

func newStubEntryRepo() *stubEntryRepo {
	return &stubEntryRepo{data: make(map[string]*domain.LibraryEntry)}
}

func (r *stubEntryRepo) Get(_ context.Context, id string) (*domain.LibraryEntry, error) {
	e, ok := r.data[id]
	if !ok {
		return nil, fmt.Errorf("not found: %w", errs.ErrNotFound)
	}
	return e, nil
}

func (r *stubEntryRepo) List(_ context.Context, _ ports.LibraryFilter) ([]*domain.LibraryEntry, int, error) {
	return nil, 0, nil
}

func (r *stubEntryRepo) Save(_ context.Context, e *domain.LibraryEntry) error {
	r.data[e.ID] = e
	return nil
}

func (r *stubEntryRepo) Delete(_ context.Context, id string) error {
	delete(r.data, id)
	return nil
}

func (r *stubEntryRepo) GetPeople(_ context.Context, _ string) ([]domain.EntryPerson, error) {
	return nil, nil
}

func (r *stubEntryRepo) SavePerson(_ context.Context, _ string, _ domain.EntryPerson) error {
	return nil
}

func (r *stubEntryRepo) RemovePerson(_ context.Context, _, _, _ string) error { return nil }

// ── Item repo stub ────────────────────────────────────────────────────────────

type stubItemRepo struct {
	items []*domain.Item
}

func (r *stubItemRepo) Get(_ context.Context, _ string) (*domain.Item, error) {
	return nil, fmt.Errorf("not found: %w", errs.ErrNotFound)
}

func (r *stubItemRepo) List(_ context.Context, _ ports.ItemFilter) ([]*domain.Item, int, error) {
	return r.items, len(r.items), nil
}

func (r *stubItemRepo) Save(_ context.Context, item *domain.Item) error {
	r.items = append(r.items, item)
	return nil
}

func (r *stubItemRepo) Delete(_ context.Context, _ string) error { return nil }

// ── Group repo stub ───────────────────────────────────────────────────────────

type stubGroupRepo struct {
	groups []*domain.Group
}

func (r *stubGroupRepo) Get(_ context.Context, id string) (*domain.Group, error) {
	for _, g := range r.groups {
		if g.ID == id {
			return g, nil
		}
	}
	return nil, fmt.Errorf("not found: %w", errs.ErrNotFound)
}

func (r *stubGroupRepo) List(_ context.Context, _ ports.GroupFilter) ([]*domain.Group, error) {
	return r.groups, nil
}

func (r *stubGroupRepo) Save(_ context.Context, g *domain.Group) error {
	r.groups = append(r.groups, g)
	return nil
}

func (r *stubGroupRepo) Delete(_ context.Context, _ string) error { return nil }

// ── Person repo stub ──────────────────────────────────────────────────────────

type stubPersonRepo struct {
	saved []*domain.Person
}

func (r *stubPersonRepo) Get(_ context.Context, id string) (*domain.Person, error) {
	for _, p := range r.saved {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("not found: %w", errs.ErrNotFound)
}

func (r *stubPersonRepo) List(_ context.Context, _ ports.PersonFilter) ([]*domain.Person, int, error) {
	return nil, 0, nil
}

func (r *stubPersonRepo) Save(_ context.Context, p *domain.Person) error {
	r.saved = append(r.saved, p)
	return nil
}

func (r *stubPersonRepo) Delete(_ context.Context, _ string) error { return nil }

// ── Tag repo stub ─────────────────────────────────────────────────────────────

type stubTagRepo struct {
	saved         []*domain.Tag
	groupTagCalls []string // "groupID:tagID" pairs recorded by AddGroupTag
}

func (r *stubTagRepo) Get(_ context.Context, _ string) (*domain.Tag, error) {
	return nil, ports.ErrNotFound
}

func (r *stubTagRepo) List(_ context.Context, f ports.TagFilter) ([]*domain.Tag, error) {
	var out []*domain.Tag
	for _, t := range r.saved {
		if f.Key != "" && t.Key != f.Key {
			continue
		}
		if f.Scope != "" && t.Scope != f.Scope {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (r *stubTagRepo) Save(_ context.Context, t *domain.Tag) error {
	if t.ID == "" {
		t.ID = fmt.Sprintf("tag-%d", len(r.saved)+1)
	}
	r.saved = append(r.saved, t)
	return nil
}

func (r *stubTagRepo) Delete(_ context.Context, _ string) error { return nil }
func (r *stubTagRepo) AddGroupTag(_ context.Context, groupID, tagID string) error {
	r.groupTagCalls = append(r.groupTagCalls, groupID+":"+tagID)
	return nil
}
func (r *stubTagRepo) RemoveGroupTag(_ context.Context, _, _ string) error { return nil }

// ── External ID repo stubs ────────────────────────────────────────────────────

// stubExternalIDRepo returns ErrNotFound for every lookup, simulating a fresh
// database where no external IDs have been imported yet.
type stubExternalIDRepo struct{}

// mapExternalIDRepo returns a seeded ID for specific "entityType:source:extID"
// keys and ErrNotFound for everything else.
type mapExternalIDRepo struct {
	entries map[string]string
}

func (r *mapExternalIDRepo) FindEntity(_ context.Context, entityType, source, value string) (string, error) {
	if id, ok := r.entries[entityType+":"+source+":"+value]; ok {
		return id, nil
	}
	return "", fmt.Errorf("not found: %w", errs.ErrNotFound)
}

func (r *stubExternalIDRepo) FindEntity(_ context.Context, _, _, _ string) (string, error) {
	return "", fmt.Errorf("not found: %w", errs.ErrNotFound)
}

type seededExternalIDRepo struct{ id string }

func (r *seededExternalIDRepo) FindEntity(_ context.Context, _, _, _ string) (string, error) {
	return r.id, nil
}

// seededArtistExternalIDRepo returns known IDs for groups and items already imported.
type seededArtistExternalIDRepo struct {
	groupIDs map[string]string // "source:extID" → internal group ID
	itemIDs  map[string]string // "source:extID" → internal item ID
}

func (r *seededArtistExternalIDRepo) FindEntity(_ context.Context, entityType, source, value string) (string, error) {
	key := source + ":" + value
	switch entityType {
	case "group":
		if id, ok := r.groupIDs[key]; ok {
			return id, nil
		}
	case "item":
		if id, ok := r.itemIDs[key]; ok {
			return id, nil
		}
	}
	return "", fmt.Errorf("not found: %w", errs.ErrNotFound)
}

// ── Job queue stub ────────────────────────────────────────────────────────────

type stubJobQueue struct {
	submitted []string
}

func (q *stubJobQueue) Submit(_ context.Context, name string, _ map[string]any, _ ports.JobFunc) (*domain.Job, error) {
	q.submitted = append(q.submitted, name)
	return &domain.Job{Name: name, Status: domain.JobStatusQueued}, nil
}

func (q *stubJobQueue) Get(_ context.Context, _ string) (*domain.Job, error) {
	return nil, ports.ErrNotFound
}

func (q *stubJobQueue) List(_ context.Context) ([]*domain.Job, error) { return nil, nil }
func (q *stubJobQueue) Cancel(_ context.Context, _ string) error      { return nil }

// ── Metadata source stubs ─────────────────────────────────────────────────────

// stubSource is a hand-rolled MetadataSource that returns a fixed scene list.
type stubSource struct {
	scenes []*domain.ExternalItem
	total  int
}

func (s *stubSource) Name() string { return "stashdb" }

func (s *stubSource) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeAdult}
}

func (s *stubSource) SearchStudios(_ context.Context, _ string, _ int) ([]*domain.ExternalStudio, error) {
	return nil, nil
}

func (s *stubSource) SearchPeople(_ context.Context, _ string, _ int) ([]*domain.ExternalPerson, error) {
	return nil, nil
}

func (s *stubSource) SearchItems(_ context.Context, _ domain.ContentType, _ string, _ int) ([]*domain.ExternalItem, error) {
	return nil, nil
}

func (s *stubSource) FindByHash(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

func (s *stubSource) FindByExternalID(_ context.Context, _ domain.ContentType, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotFound
}

func (s *stubSource) FetchEntryContent(_ context.Context, _ domain.ContentType, _ string, page, _ int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	if page == 1 {
		return nil, s.scenes, s.total, nil
	}
	return nil, nil, s.total, nil
}

func (s *stubSource) FetchGroupContent(_ context.Context, _ domain.ContentType, _ string, _, _ int) ([]*domain.ExternalItem, int, error) {
	return nil, 0, ports.ErrNotSupported
}

func (s *stubSource) FetchEntryPeople(_ context.Context, _ string) ([]*domain.ExternalPerson, error) {
	return nil, ports.ErrNotSupported
}

// stubMusicSource returns albums via FetchEntryContent and per-album tracks via
// FetchGroupContent. Name() returns "mbz" to match artist entry external IDs.
type stubMusicSource struct {
	albums   []*domain.ExternalGroup
	tracks   map[string][]*domain.ExternalItem // groupExternalID → tracks
	findItem *domain.ExternalItem              // if non-nil, returned by FindByExternalID
	people   []*domain.ExternalPerson          // if non-nil, returned by FetchEntryPeople
}

func (s *stubMusicSource) Name() string { return "mbz" }

func (s *stubMusicSource) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

func (s *stubMusicSource) SearchStudios(_ context.Context, _ string, _ int) ([]*domain.ExternalStudio, error) {
	return nil, nil
}

func (s *stubMusicSource) SearchPeople(_ context.Context, _ string, _ int) ([]*domain.ExternalPerson, error) {
	return nil, nil
}

func (s *stubMusicSource) SearchItems(_ context.Context, _ domain.ContentType, _ string, _ int) ([]*domain.ExternalItem, error) {
	return nil, nil
}

func (s *stubMusicSource) FindByHash(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

func (s *stubMusicSource) FindByExternalID(_ context.Context, _ domain.ContentType, _ string) (*domain.ExternalItem, error) {
	if s.findItem != nil {
		return s.findItem, nil
	}
	return nil, ports.ErrNotFound
}

func (s *stubMusicSource) FetchEntryContent(_ context.Context, _ domain.ContentType, _ string, page, _ int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	if page == 1 {
		return s.albums, nil, len(s.albums), nil
	}
	return nil, nil, len(s.albums), nil
}

func (s *stubMusicSource) FetchGroupContent(_ context.Context, _ domain.ContentType, groupExtID string, page, _ int) ([]*domain.ExternalItem, int, error) {
	tracks := s.tracks[groupExtID]
	if page == 1 {
		return tracks, len(tracks), nil
	}
	return nil, len(tracks), nil
}

func (s *stubMusicSource) FetchEntryPeople(_ context.Context, _ string) ([]*domain.ExternalPerson, error) {
	if s.people != nil {
		return s.people, nil
	}
	return nil, ports.ErrNotSupported
}

// ── Image downloader stub ─────────────────────────────────────────────────────

type stubImageDownloader struct {
	ext   string
	calls []string
}

func (d *stubImageDownloader) Download(_ context.Context, url, _, _ string) string {
	d.calls = append(d.calls, url)
	return d.ext
}

// ── Image-only source stub ────────────────────────────────────────────────────

// stubImageSource is a configurable metadata source that returns a fixed item
// from FindByExternalID. contentTypes defaults to [music] when empty.
type stubImageSource struct {
	sourceName   string
	contentTypes []domain.ContentType
	findItem     *domain.ExternalItem
	findErr      error
}

func (s *stubImageSource) Name() string { return s.sourceName }
func (s *stubImageSource) ContentTypes() []domain.ContentType {
	if len(s.contentTypes) > 0 {
		return s.contentTypes
	}
	return []domain.ContentType{domain.ContentTypeMusic}
}

func (s *stubImageSource) SearchStudios(_ context.Context, _ string, _ int) ([]*domain.ExternalStudio, error) {
	return nil, nil
}

func (s *stubImageSource) SearchPeople(_ context.Context, _ string, _ int) ([]*domain.ExternalPerson, error) {
	return nil, nil
}

func (s *stubImageSource) SearchItems(_ context.Context, _ domain.ContentType, _ string, _ int) ([]*domain.ExternalItem, error) {
	return nil, nil
}

func (s *stubImageSource) FindByHash(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

func (s *stubImageSource) FindByExternalID(_ context.Context, _ domain.ContentType, _ string) (*domain.ExternalItem, error) {
	return s.findItem, s.findErr
}

func (s *stubImageSource) FetchEntryContent(_ context.Context, _ domain.ContentType, _ string, _, _ int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	return nil, nil, 0, nil
}

func (s *stubImageSource) FetchGroupContent(_ context.Context, _ domain.ContentType, _ string, _, _ int) ([]*domain.ExternalItem, int, error) {
	return nil, 0, ports.ErrNotSupported
}

func (s *stubImageSource) FetchEntryPeople(_ context.Context, _ string) ([]*domain.ExternalPerson, error) {
	return nil, ports.ErrNotSupported
}
