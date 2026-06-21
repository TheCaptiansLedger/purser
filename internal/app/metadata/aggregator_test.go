package metadata_test

import (
	"context"
	"errors"
	"purser/internal/app/metadata"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type aggStubSource struct {
	name             string
	contentTypes     []domain.ContentType
	findItem         *domain.ExternalItem
	findErr          error
	searchItems      []*domain.ExternalItem
	searchErr        error
	contentItems     []*domain.ExternalItem
	contentErr       error
	searchStudios    []*domain.ExternalStudio
	searchStudiosErr error
	searchPeople     []*domain.ExternalPerson
	searchPeopleErr  error
}

func (s *aggStubSource) Name() string {
	return s.name
}

func (s *aggStubSource) ContentTypes() []domain.ContentType {
	return s.contentTypes
}

func (s *aggStubSource) SearchStudios(_ context.Context, _ string, _ int) ([]*domain.ExternalStudio, error) {
	return s.searchStudios, s.searchStudiosErr
}

func (s *aggStubSource) SearchPeople(_ context.Context, _ string, _ int) ([]*domain.ExternalPerson, error) {
	return s.searchPeople, s.searchPeopleErr
}

func (s *aggStubSource) SearchItems(_ context.Context, _ domain.ContentType, _ string, _ int) ([]*domain.ExternalItem, error) {
	return s.searchItems, s.searchErr
}

func (s *aggStubSource) FindByHash(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

func (s *aggStubSource) FindByExternalID(_ context.Context, _ domain.ContentType, _ string) (*domain.ExternalItem, error) {
	return s.findItem, s.findErr
}

func (s *aggStubSource) FetchEntryContent(_ context.Context, _ domain.ContentType, _ string, _, _ int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	return nil, s.contentItems, len(s.contentItems), s.contentErr
}

func (s *aggStubSource) FetchGroupContent(_ context.Context, _ domain.ContentType, _ string, _, _ int) ([]*domain.ExternalItem, int, error) {
	return nil, 0, ports.ErrNotSupported
}

func (s *aggStubSource) FetchEntryPeople(_ context.Context, _ string) ([]*domain.ExternalPerson, error) {
	return nil, ports.ErrNotSupported
}

// ── FindByExternalID ──────────────────────────────────────────────────────────

func TestAggregator_FindByExternalID_MergesInPriorityOrder(t *testing.T) {
	primary := &aggStubSource{
		name:         "primary",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		findItem: &domain.ExternalItem{
			Source:      "primary",
			Title:       "Radiohead",
			ContentType: domain.ContentTypeMusic,
		},
	}
	supplemental := &aggStubSource{
		name:         "supplemental",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		findItem: &domain.ExternalItem{
			Source: "supplemental",
			Images: []domain.ExternalImage{
				{Type: domain.ImageTypeBanner, URL: "https://example.com/banner.jpg"},
			},
		},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{primary, supplemental})

	item, err := agg.FindByExternalID(context.Background(), domain.ContentTypeMusic, "some-id", "some-entity-id")
	if err != nil {
		t.Fatalf("FindByExternalID: %v", err)
	}

	if item.Title != "Radiohead" {
		t.Errorf("Title = %q, want Radiohead (primary wins on scalar fields)", item.Title)
	}
	if len(item.Images) == 0 {
		t.Error("Images should include supplemental source images")
	}
	if item.Images[0].Source != "supplemental" {
		t.Errorf("Images[0].Source = %q, want supplemental (stamped by merge)", item.Images[0].Source)
	}
}

func TestAggregator_FindByExternalID_PrimaryFails_SupplementalReturned(t *testing.T) {
	primary := &aggStubSource{
		name:         "primary",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		findErr:      errors.New("primary down"),
	}
	supplemental := &aggStubSource{
		name:         "supplemental",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		findItem: &domain.ExternalItem{
			Source: "supplemental",
			Title:  "Fallback Title",
		},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{primary, supplemental})

	item, err := agg.FindByExternalID(context.Background(), domain.ContentTypeMusic, "some-id", "some-entity-id")
	if err != nil {
		t.Fatalf("FindByExternalID should not error when a supplemental source succeeds: %v", err)
	}
	if item.Title != "Fallback Title" {
		t.Errorf("Title = %q, want Fallback Title", item.Title)
	}
}

func TestAggregator_FindByExternalID_AllFail_ReturnsError(t *testing.T) {
	primary := &aggStubSource{
		name:         "primary",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		findErr:      errors.New("primary down"),
	}
	supplemental := &aggStubSource{
		name:         "supplemental",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		findErr:      errors.New("supplemental down"),
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{primary, supplemental})

	_, err := agg.FindByExternalID(context.Background(), domain.ContentTypeMusic, "some-id", "some-entity-id")
	if err == nil {
		t.Fatal("expected error when all sources fail, got nil")
	}
}

func TestAggregator_FindByExternalID_NoMatchingSources_ReturnsError(t *testing.T) {
	src := &aggStubSource{
		name:         "tmdb",
		contentTypes: []domain.ContentType{domain.ContentTypeMovie},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{src})

	_, err := agg.FindByExternalID(context.Background(), domain.ContentTypeMusic, "some-id", "some-entity-id")
	if err == nil {
		t.Fatal("expected error when no sources match content type, got nil")
	}
}

// ── SearchItems ───────────────────────────────────────────────────────────────

func TestAggregator_SearchItems_CombinesAcrossSources(t *testing.T) {
	src1 := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		searchItems: []*domain.ExternalItem{
			{Source: "mbz", Title: "Creep"},
		},
	}
	src2 := &aggStubSource{
		name:         "lastfm",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		searchItems: []*domain.ExternalItem{
			{Source: "lastfm", Title: "Creep"},
			{Source: "lastfm", Title: "Karma Police"},
		},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{src1, src2})

	items, err := agg.SearchItems(context.Background(), domain.ContentTypeMusic, "creep", 10)
	if err != nil {
		t.Fatalf("SearchItems: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("item count = %d, want 3 (1 from mbz + 2 from lastfm)", len(items))
	}
}

func TestAggregator_SearchItems_SourceErrorSkipped(t *testing.T) {
	good := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		searchItems:  []*domain.ExternalItem{{Source: "mbz", Title: "Creep"}},
	}
	bad := &aggStubSource{
		name:         "lastfm",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		searchErr:    errors.New("service unavailable"),
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{good, bad})

	items, err := agg.SearchItems(context.Background(), domain.ContentTypeMusic, "creep", 10)
	if err != nil {
		t.Fatalf("SearchItems should not return error when one source fails: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("item count = %d, want 1 (only from good source)", len(items))
	}
}

// ── SearchStudios ─────────────────────────────────────────────────────────────

func TestAggregator_SearchStudios_FanOut(t *testing.T) {
	src1 := &aggStubSource{
		name: "stashdb", contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		searchStudios: []*domain.ExternalStudio{{Name: "Studio A"}},
	}
	src2 := &aggStubSource{
		name: "tpdb", contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		searchStudios: []*domain.ExternalStudio{{Name: "Studio B"}, {Name: "Studio C"}},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{src1, src2})

	studios, err := agg.SearchStudios(context.Background(), "studio", domain.ContentTypeAdult, 10)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(studios) != 3 {
		t.Errorf("studio count = %d, want 3 (1 from src1 + 2 from src2)", len(studios))
	}
}

func TestAggregator_SearchStudios_SourceError(t *testing.T) {
	good := &aggStubSource{
		name: "stashdb", contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		searchStudios: []*domain.ExternalStudio{{Name: "Studio A"}},
	}
	bad := &aggStubSource{
		name: "tpdb", contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		searchStudiosErr: errors.New("service unavailable"),
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{good, bad})

	studios, err := agg.SearchStudios(context.Background(), "studio", domain.ContentTypeAdult, 10)
	if err != nil {
		t.Fatalf("SearchStudios should not error when one source fails: %v", err)
	}
	if len(studios) != 1 {
		t.Errorf("studio count = %d, want 1 (only from good source)", len(studios))
	}
}

func TestAggregator_SearchStudios_NoMatchingSources(t *testing.T) {
	agg := metadata.NewAggregator(nil)
	studios, err := agg.SearchStudios(context.Background(), "studio", domain.ContentTypeAdult, 10)
	if err != nil {
		t.Fatalf("SearchStudios with no sources: %v", err)
	}
	if len(studios) != 0 {
		t.Errorf("studio count = %d, want 0", len(studios))
	}
}

// ── SearchPeople ──────────────────────────────────────────────────────────────

func TestAggregator_SearchPeople_FanOut(t *testing.T) {
	src1 := &aggStubSource{
		name: "stashdb", contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		searchPeople: []*domain.ExternalPerson{{Name: "Alice"}},
	}
	src2 := &aggStubSource{
		name: "tpdb", contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		searchPeople: []*domain.ExternalPerson{{Name: "Bob"}, {Name: "Carol"}},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{src1, src2})

	people, err := agg.SearchPeople(context.Background(), "query", domain.ContentTypeAdult, 10)
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}
	if len(people) != 3 {
		t.Errorf("people count = %d, want 3 (1 from src1 + 2 from src2)", len(people))
	}
}

func TestAggregator_SearchPeople_SourceError(t *testing.T) {
	good := &aggStubSource{
		name: "stashdb", contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		searchPeople: []*domain.ExternalPerson{{Name: "Alice"}},
	}
	bad := &aggStubSource{
		name: "tpdb", contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		searchPeopleErr: errors.New("service unavailable"),
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{good, bad})

	people, err := agg.SearchPeople(context.Background(), "query", domain.ContentTypeAdult, 10)
	if err != nil {
		t.Fatalf("SearchPeople should not error when one source fails: %v", err)
	}
	if len(people) != 1 {
		t.Errorf("people count = %d, want 1 (only from good source)", len(people))
	}
}

// ── FetchEntryContent ─────────────────────────────────────────────────────────

func TestAggregator_FetchEntryContent_CombinesAcrossSources(t *testing.T) {
	src1 := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		contentItems: []*domain.ExternalItem{
			{Source: "mbz", Title: "OK Computer"},
		},
	}
	src2 := &aggStubSource{
		name:         "fanart",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		contentItems: []*domain.ExternalItem{
			{Source: "fanart", Title: "OK Computer", Images: []domain.ExternalImage{{Type: domain.ImageTypePoster, URL: "https://example.com/ok.jpg"}}},
			{Source: "fanart", Title: "The Bends", Images: []domain.ExternalImage{{Type: domain.ImageTypePoster, URL: "https://example.com/bends.jpg"}}},
		},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{src1, src2})

	items, err := agg.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "radiohead-id")
	if err != nil {
		t.Fatalf("FetchEntryContent: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("item count = %d, want 3 (1 from mbz + 2 from fanart)", len(items))
	}
}
