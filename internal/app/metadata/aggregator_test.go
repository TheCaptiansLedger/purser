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
	personRoles      []domain.PersonRole
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
	groupItem        *domain.ExternalItem
	groupErr         error
	imagePriority    int
	personImageURL   string
	personImageErr   error
}

func (s *aggStubSource) PersonRoles() []domain.PersonRole { return s.personRoles }

func (s *aggStubSource) Name() string       { return s.name }
func (s *aggStubSource) ImagePriority() int { return s.imagePriority }

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

func (s *aggStubSource) FindGroupImages(_ context.Context, _ domain.ContentType, _, _ string) (*domain.ExternalItem, error) {
	if s.groupErr != nil {
		return nil, s.groupErr
	}
	if s.groupItem != nil {
		return s.groupItem, nil
	}
	return nil, ports.ErrNotSupported
}

func (s *aggStubSource) FetchPersonImage(_ context.Context, _ string) (*domain.ExternalImage, error) {
	if s.personImageErr != nil {
		return nil, s.personImageErr
	}
	if s.personImageURL != "" {
		return &domain.ExternalImage{URL: s.personImageURL}, nil
	}
	return nil, ports.ErrNotSupported
}

// personImageOnlyStub implements MetadataSource + PersonImageSource but NOT
// PersonRoleSource, exactly like the real TheAudioDB adapter.
type personImageOnlyStub struct {
	name     string
	imageURL string
	imageErr error
}

func (s *personImageOnlyStub) Name() string                       { return s.name }
func (s *personImageOnlyStub) ContentTypes() []domain.ContentType { return nil }
func (s *personImageOnlyStub) ImagePriority() int                 { return 0 }
func (s *personImageOnlyStub) FetchPersonImage(_ context.Context, _ string) (*domain.ExternalImage, error) {
	if s.imageErr != nil {
		return nil, s.imageErr
	}
	if s.imageURL != "" {
		return &domain.ExternalImage{URL: s.imageURL}, nil
	}
	return nil, ports.ErrNotFound
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

func TestAggregator_SearchStudios_DeduplicatesByMBID(t *testing.T) {
	mbz := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		searchStudios: []*domain.ExternalStudio{
			{Source: domain.SourceMusicBrainz, ExternalID: "mbid-whitesnake", Name: "Whitesnake"},
		},
	}
	audiodb := &aggStubSource{
		name:         "audiodb",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		searchStudios: []*domain.ExternalStudio{
			{Source: domain.SourceTheAudioDB, ExternalID: "mbid-whitesnake", Name: "Whitesnake"},
		},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{mbz, audiodb})

	studios, err := agg.SearchStudios(context.Background(), "whitesnake", domain.ContentTypeMusic, 10)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(studios) != 1 {
		t.Errorf("studio count = %d, want 1 (duplicate MBID deduplicated)", len(studios))
	}
}

func TestAggregator_SearchStudios_MBZSourceWithAudioDBImage(t *testing.T) {
	// When both sources return the same MBID, the MBZ entry is kept as the
	// canonical source (it supports album fetching). The audiodb image URL is
	// applied to the MBZ entry so the UI can show a thumbnail.
	mbz := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		searchStudios: []*domain.ExternalStudio{
			{Source: domain.SourceMusicBrainz, ExternalID: "mbid-whitesnake", Name: "Whitesnake"},
		},
	}
	audiodb := &aggStubSource{
		name:          "audiodb",
		contentTypes:  []domain.ContentType{domain.ContentTypeMusic},
		imagePriority: 100,
		searchStudios: []*domain.ExternalStudio{
			{Source: domain.SourceTheAudioDB, ExternalID: "mbid-whitesnake", Name: "Whitesnake", ImageURL: "https://audiodb.example.com/thumb.jpg"},
		},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{mbz, audiodb})

	studios, err := agg.SearchStudios(context.Background(), "whitesnake", domain.ContentTypeMusic, 10)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(studios) != 1 {
		t.Fatalf("studio count = %d, want 1", len(studios))
	}
	if studios[0].Source != domain.SourceMusicBrainz {
		t.Errorf("Source = %q, want MusicBrainz (canonical source for album fetching)", studios[0].Source)
	}
	if studios[0].ImageURL != "https://audiodb.example.com/thumb.jpg" {
		t.Errorf("ImageURL = %q, want audiodb thumbnail", studios[0].ImageURL)
	}
}

func TestAggregator_SearchStudios_AudioDBFirstStillPromotesMBZ(t *testing.T) {
	// audiodb result arrives before mbz in the combined slice (simulates the case
	// where audiodb is priority 0 or its goroutine finishes first).
	audiodb := &aggStubSource{
		name:          "audiodb",
		contentTypes:  []domain.ContentType{domain.ContentTypeMusic},
		imagePriority: 100,
		searchStudios: []*domain.ExternalStudio{
			{Source: domain.SourceTheAudioDB, ExternalID: "mbid-whitesnake", Name: "Whitesnake", ImageURL: "https://audiodb.example.com/thumb.jpg"},
		},
	}
	mbz := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		searchStudios: []*domain.ExternalStudio{
			{Source: domain.SourceMusicBrainz, ExternalID: "mbid-whitesnake", Name: "Whitesnake"},
		},
	}
	// audiodb is first in sources so its results appear first in the combined slice.
	agg := metadata.NewAggregator([]ports.MetadataSource{audiodb, mbz})

	studios, err := agg.SearchStudios(context.Background(), "whitesnake", domain.ContentTypeMusic, 10)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(studios) != 1 {
		t.Fatalf("studio count = %d, want 1", len(studios))
	}
	if studios[0].Source != domain.SourceMusicBrainz {
		t.Errorf("Source = %q, want MusicBrainz even when audiodb arrives first", studios[0].Source)
	}
	if studios[0].ImageURL != "https://audiodb.example.com/thumb.jpg" {
		t.Errorf("ImageURL = %q, want audiodb thumbnail preserved after promotion", studios[0].ImageURL)
	}
}

func TestAggregator_SearchStudios_PreservesDistinctMBIDs(t *testing.T) {
	mbz := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		searchStudios: []*domain.ExternalStudio{
			{Source: domain.SourceMusicBrainz, ExternalID: "mbid-whitesnake", Name: "Whitesnake"},
			{Source: domain.SourceMusicBrainz, ExternalID: "mbid-deep-purple", Name: "Deep Purple"},
		},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{mbz})

	studios, err := agg.SearchStudios(context.Background(), "rock", domain.ContentTypeMusic, 10)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(studios) != 2 {
		t.Errorf("studio count = %d, want 2 (distinct MBIDs preserved)", len(studios))
	}
}

func TestAggregator_SearchStudios_StudioOnlySourceContributes(t *testing.T) {
	// A source that implements only StudioSearchSource (not PeopleSearchSource or
	// ItemSearchSource) must still participate in the studio search fan-out.
	// This is the TheAudioDB case: it can search studios but not people or items.
	src := &studioSearchOnlyStub{
		name: "audiodb",
		ct:   domain.ContentTypeMusic,
		studios: []*domain.ExternalStudio{
			{Source: domain.SourceTheAudioDB, ExternalID: "mbid-1", Name: "Radiohead", ImageURL: "https://audiodb.example.com/radiohead.jpg"},
		},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{src})

	studios, err := agg.SearchStudios(context.Background(), "radiohead", domain.ContentTypeMusic, 10)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(studios) != 1 {
		t.Fatalf("studio count = %d, want 1", len(studios))
	}
	if studios[0].ImageURL != "https://audiodb.example.com/radiohead.jpg" {
		t.Errorf("ImageURL = %q, want audiodb image URL", studios[0].ImageURL)
	}
}

// studioSearchOnlyStub satisfies MetadataSource and StudioSearchSource but
// deliberately does not implement PeopleSearchSource or ItemSearchSource.
type studioSearchOnlyStub struct {
	name    string
	ct      domain.ContentType
	studios []*domain.ExternalStudio
}

func (s *studioSearchOnlyStub) Name() string                       { return s.name }
func (s *studioSearchOnlyStub) ContentTypes() []domain.ContentType { return []domain.ContentType{s.ct} }
func (s *studioSearchOnlyStub) ImagePriority() int                 { return 100 }
func (s *studioSearchOnlyStub) SearchStudios(_ context.Context, _ string, _ int) ([]*domain.ExternalStudio, error) {
	return s.studios, nil
}

// studioSearchAndThumbStub implements StudioSearchSource and StudioThumbSource
// but not PeopleSearchSource or ItemSearchSource. Used to test thumb enrichment.
type studioSearchAndThumbStub struct {
	name       string
	ct         domain.ContentType
	imagePri   int
	studios    []*domain.ExternalStudio
	thumbsByID map[string]string
}

func (s *studioSearchAndThumbStub) Name() string { return s.name }
func (s *studioSearchAndThumbStub) ContentTypes() []domain.ContentType {
	return []domain.ContentType{s.ct}
}
func (s *studioSearchAndThumbStub) ImagePriority() int { return s.imagePri }
func (s *studioSearchAndThumbStub) SearchStudios(_ context.Context, _ string, _ int) ([]*domain.ExternalStudio, error) {
	return s.studios, nil
}

func (s *studioSearchAndThumbStub) FetchStudioThumb(_ context.Context, id string) (string, error) {
	if url, ok := s.thumbsByID[id]; ok {
		return url, nil
	}
	return "", ports.ErrNotFound
}

func TestAggregator_SearchStudios_ThumbEnrichmentFillsMissingImages(t *testing.T) {
	// MBZ returns two artists (no images). TheAudioDB text search returns nothing
	// but has FetchStudioThumb. Enrichment must fill images for both results.
	mbz := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		searchStudios: []*domain.ExternalStudio{
			{Source: domain.SourceMusicBrainz, ExternalID: "mbid-1", Name: "Radiohead"},
			{Source: domain.SourceMusicBrainz, ExternalID: "mbid-2", Name: "Portishead"},
		},
	}
	audiodb := &studioSearchAndThumbStub{
		name:     "audiodb",
		ct:       domain.ContentTypeMusic,
		imagePri: 100,
		thumbsByID: map[string]string{
			"mbid-1": "https://audiodb.example.com/radiohead.jpg",
			"mbid-2": "https://audiodb.example.com/portishead.jpg",
		},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{mbz, audiodb})

	studios, err := agg.SearchStudios(context.Background(), "music", domain.ContentTypeMusic, 10)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(studios) != 2 {
		t.Fatalf("studio count = %d, want 2", len(studios))
	}
	for _, s := range studios {
		if s.ImageURL == "" {
			t.Errorf("studio %q: ImageURL empty after enrichment", s.Name)
		}
	}
}

func TestAggregator_SearchStudios_ThumbEnrichmentSkipsExistingImage(t *testing.T) {
	// A studio that already has an image (from dedup) must not be overwritten.
	existing := "https://existing.example.com/image.jpg"
	mbz := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		searchStudios: []*domain.ExternalStudio{
			{Source: domain.SourceMusicBrainz, ExternalID: "mbid-1", Name: "Radiohead", ImageURL: existing},
		},
	}
	audiodb := &studioSearchAndThumbStub{
		name:     "audiodb",
		ct:       domain.ContentTypeMusic,
		imagePri: 100,
		thumbsByID: map[string]string{
			"mbid-1": "https://audiodb.example.com/different.jpg",
		},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{mbz, audiodb})

	studios, err := agg.SearchStudios(context.Background(), "radiohead", domain.ContentTypeMusic, 10)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if studios[0].ImageURL != existing {
		t.Errorf("ImageURL = %q, want existing image preserved", studios[0].ImageURL)
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

	people, err := agg.SearchPeople(context.Background(), "query", "", 10)
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

	people, err := agg.SearchPeople(context.Background(), "query", "", 10)
	if err != nil {
		t.Fatalf("SearchPeople should not error when one source fails: %v", err)
	}
	if len(people) != 1 {
		t.Errorf("people count = %d, want 1 (only from good source)", len(people))
	}
}

func TestAggregator_SearchPeople_RoleFilter(t *testing.T) {
	performer := &aggStubSource{
		name:         "stashdb",
		personRoles:  []domain.PersonRole{domain.RolePerformer, domain.RoleActress},
		searchPeople: []*domain.ExternalPerson{{Name: "Alice"}},
	}
	musician := &aggStubSource{
		name:         "mbz",
		personRoles:  []domain.PersonRole{domain.RoleArtist},
		searchPeople: []*domain.ExternalPerson{{Name: "Bob"}},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{performer, musician})

	people, err := agg.SearchPeople(context.Background(), "query", domain.RolePerformer, 10)
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}
	if len(people) != 1 || people[0].Name != "Alice" {
		t.Errorf("got %v, want only Alice from performer source", people)
	}
}

func TestAggregator_SearchPeople_NoRoleFilter_QueriesAll(t *testing.T) {
	performer := &aggStubSource{
		name:         "stashdb",
		personRoles:  []domain.PersonRole{domain.RolePerformer},
		searchPeople: []*domain.ExternalPerson{{Name: "Alice"}},
	}
	musician := &aggStubSource{
		name:         "mbz",
		personRoles:  []domain.PersonRole{domain.RoleArtist},
		searchPeople: []*domain.ExternalPerson{{Name: "Bob"}},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{performer, musician})

	people, err := agg.SearchPeople(context.Background(), "query", "", 10)
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}
	if len(people) != 2 {
		t.Errorf("people count = %d, want 2 (all sources when no role filter)", len(people))
	}
}

func TestAggregator_SearchPeople_EnrichesImages(t *testing.T) {
	// MusicBrainz-like: returns artist with ExternalID but no image.
	searchSrc := &aggStubSource{
		name:        "mbz",
		personRoles: []domain.PersonRole{domain.RoleArtist},
		searchPeople: []*domain.ExternalPerson{
			{Name: "Stevie Nicks", ExternalID: "mbid-stevie", Source: domain.SourceMusicBrainz},
		},
	}
	// TheAudioDB-like: does NOT implement PersonRoleSource (no PersonRoles method),
	// so it is always a catch-all for enrichment even when a role filter is applied.
	imageSrc := &personImageOnlyStub{
		name:     "audiodb",
		imageURL: "https://img.audiodb.com/stevie.jpg",
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{searchSrc, imageSrc})

	people, err := agg.SearchPeople(context.Background(), "Stevie Nicks", domain.RoleArtist, 10)
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}
	if len(people) != 1 {
		t.Fatalf("people count = %d, want 1", len(people))
	}
	if people[0].ImageURL == "" {
		t.Error("ImageURL should be enriched from PersonImageSource but is empty")
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

// ── FetchGroupImages ──────────────────────────────────────────────────────────

func TestAggregator_FetchGroupImages_ReturnsImagesFromSupportingSource(t *testing.T) {
	src := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		groupItem: &domain.ExternalItem{
			Images: []domain.ExternalImage{
				{Type: domain.ImageTypePoster, URL: "https://example.com/cover.jpg"},
			},
		},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{src})

	entryExtIDs := []domain.ExternalID{{Source: "mbz", Value: "artist-mbid"}}
	groupExtIDs := []domain.ExternalID{{Source: "mbz", Value: "rg-mbid"}}
	images := agg.FetchGroupImages(context.Background(), domain.ContentTypeMusic, entryExtIDs, groupExtIDs)

	if len(images) != 1 {
		t.Fatalf("image count = %d, want 1", len(images))
	}
	if images[0].URL != "https://example.com/cover.jpg" {
		t.Errorf("URL = %q, want cover.jpg", images[0].URL)
	}
}

func TestAggregator_FetchGroupImages_SkipsErrNotSupported(t *testing.T) {
	src := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		groupErr:     ports.ErrNotSupported,
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{src})

	entryExtIDs := []domain.ExternalID{{Source: "mbz", Value: "artist-mbid"}}
	groupExtIDs := []domain.ExternalID{{Source: "mbz", Value: "rg-mbid"}}
	images := agg.FetchGroupImages(context.Background(), domain.ContentTypeMusic, entryExtIDs, groupExtIDs)

	if len(images) != 0 {
		t.Errorf("image count = %d, want 0 (ErrNotSupported should be silently skipped)", len(images))
	}
}

func TestAggregator_FetchGroupImages_SkipsErrNotFound(t *testing.T) {
	src := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		groupErr:     ports.ErrNotFound,
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{src})

	entryExtIDs := []domain.ExternalID{{Source: "mbz", Value: "artist-mbid"}}
	groupExtIDs := []domain.ExternalID{{Source: "mbz", Value: "rg-mbid"}}
	images := agg.FetchGroupImages(context.Background(), domain.ContentTypeMusic, entryExtIDs, groupExtIDs)

	if len(images) != 0 {
		t.Errorf("image count = %d, want 0 (ErrNotFound should be silently skipped)", len(images))
	}
}

func TestAggregator_FetchGroupImages_DeduplicatesByURL(t *testing.T) {
	sharedURL := "https://example.com/shared.jpg"
	src1 := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		groupItem: &domain.ExternalItem{
			Images: []domain.ExternalImage{
				{Type: domain.ImageTypePoster, URL: sharedURL},
				{Type: domain.ImageTypePoster, URL: "https://example.com/unique.jpg"},
			},
		},
	}
	src2 := &aggStubSource{
		name:         "fanart",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		groupItem: &domain.ExternalItem{
			Images: []domain.ExternalImage{
				{Type: domain.ImageTypePoster, URL: sharedURL}, // duplicate
			},
		},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{src1, src2})

	entryExtIDs := []domain.ExternalID{{Source: "mbz", Value: "artist-mbid"}}
	groupExtIDs := []domain.ExternalID{{Source: "mbz", Value: "rg-mbid"}}
	images := agg.FetchGroupImages(context.Background(), domain.ContentTypeMusic, entryExtIDs, groupExtIDs)

	if len(images) != 2 {
		t.Errorf("image count = %d, want 2 (shared URL deduplicated)", len(images))
	}
}

func TestAggregator_FetchGroupImages_PrefersSourceNativeID(t *testing.T) {
	// When the entry has both a source-native ID and another source's ID,
	// the aggregator should use the source-native one.
	var receivedParent string
	src := &aggStubSource{
		name:         "fanart",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
	}
	// Wrap FindGroupImages to capture which parentExtID was passed.
	capturingSrc := &capturingGroupImageSource{
		aggStubSource: src,
		onCall: func(parent, _ string) {
			receivedParent = parent
		},
		item: &domain.ExternalItem{Images: nil},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{capturingSrc})

	entryExtIDs := []domain.ExternalID{
		{Source: "mbz", Value: "mbz-artist-id"},
		{Source: "fanart", Value: "fanart-native-id"},
	}
	groupExtIDs := []domain.ExternalID{{Source: "fanart", Value: "fanart-rg-id"}}
	agg.FetchGroupImages(context.Background(), domain.ContentTypeMusic, entryExtIDs, groupExtIDs)

	if receivedParent != "fanart-native-id" {
		t.Errorf("parentExtID = %q, want fanart-native-id (source-native preferred over fallback)", receivedParent)
	}
}

func TestAggregator_FetchGroupImages_FallsBackToFirstAvailableID(t *testing.T) {
	// When no source-native ID exists, the aggregator falls back to the first
	// available ExternalID so that sources sharing an ID namespace (e.g. fanart
	// using MBZ MBIDs) are still called.
	var receivedParent string
	src := &aggStubSource{
		name:         "fanart",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
	}
	capturingSrc := &capturingGroupImageSource{
		aggStubSource: src,
		onCall: func(parent, _ string) {
			receivedParent = parent
		},
		item: &domain.ExternalItem{Images: nil},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{capturingSrc})

	// Entry has only an MBZ ExternalID — no fanart-native one.
	entryExtIDs := []domain.ExternalID{{Source: "mbz", Value: "mbz-artist-id"}}
	groupExtIDs := []domain.ExternalID{{Source: "mbz", Value: "mbz-rg-id"}}
	agg.FetchGroupImages(context.Background(), domain.ContentTypeMusic, entryExtIDs, groupExtIDs)

	if receivedParent != "mbz-artist-id" {
		t.Errorf("parentExtID = %q, want mbz-artist-id (first-available fallback)", receivedParent)
	}
}

func TestAggregator_FetchGroupImages_EmptyExtIDs_ReturnsNoImages(t *testing.T) {
	src := &aggStubSource{
		name:         "mbz",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		groupItem:    &domain.ExternalItem{Images: []domain.ExternalImage{{URL: "https://example.com/cover.jpg"}}},
	}
	agg := metadata.NewAggregator([]ports.MetadataSource{src})

	images := agg.FetchGroupImages(context.Background(), domain.ContentTypeMusic, nil, nil)
	if len(images) != 0 {
		t.Errorf("image count = %d, want 0 when no external IDs provided", len(images))
	}
}

// capturingGroupImageSource wraps aggStubSource and records the IDs passed to FindGroupImages.
type capturingGroupImageSource struct {
	*aggStubSource
	onCall func(parentExtID, groupExtID string)
	item   *domain.ExternalItem
}

func (s *capturingGroupImageSource) FindGroupImages(_ context.Context, _ domain.ContentType, parentExtID, groupExtID string) (*domain.ExternalItem, error) {
	if s.onCall != nil {
		s.onCall(parentExtID, groupExtID)
	}
	return s.item, nil
}
