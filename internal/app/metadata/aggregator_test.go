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
	name         string
	contentTypes []domain.ContentType
	findItem     *domain.ExternalItem
	findErr      error
	searchItems  []*domain.ExternalItem
	searchErr    error
	contentItems []*domain.ExternalItem
	contentErr   error
}

func (s *aggStubSource) Name() string {
	return s.name
}

func (s *aggStubSource) ContentTypes() []domain.ContentType {
	return s.contentTypes
}

func (s *aggStubSource) SearchStudios(_ context.Context, _ string, _ int) ([]*domain.ExternalStudio, error) {
	return nil, ports.ErrNotSupported
}

func (s *aggStubSource) SearchPeople(_ context.Context, _ string, _ int) ([]*domain.ExternalPerson, error) {
	return nil, ports.ErrNotSupported
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

type stubImageRepo struct {
	stored    []domain.StoredImage
	upsertErr error
}

func (r *stubImageRepo) Upsert(_ context.Context, images []domain.StoredImage) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.stored = append(r.stored, images...)
	return nil
}

func (r *stubImageRepo) List(_ context.Context, entityType, entityID string, imageType *domain.ImageType) ([]domain.StoredImage, error) {
	var out []domain.StoredImage
	for _, img := range r.stored {
		if img.EntityType == entityType && img.EntityID == entityID {
			if imageType == nil || img.ImageType == *imageType {
				out = append(out, img)
			}
		}
	}
	return out, nil
}

func (r *stubImageRepo) GetSelection(_ context.Context, _, _ string, _ domain.ImageType) (*domain.StoredImage, error) {
	return nil, nil //nolint:nilnil // stub: no selection is a valid state for tests
}

func (r *stubImageRepo) SetSelection(_ context.Context, _, _ string, _ domain.ImageType, _ string) error {
	return nil
}

func (r *stubImageRepo) ClearSelection(_ context.Context, _, _ string, _ domain.ImageType) error {
	return nil
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
	imageRepo := &stubImageRepo{}
	agg := metadata.NewAggregator([]ports.MetadataSource{primary, supplemental}, imageRepo)

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
	agg := metadata.NewAggregator([]ports.MetadataSource{primary, supplemental}, &stubImageRepo{})

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
	agg := metadata.NewAggregator([]ports.MetadataSource{primary, supplemental}, &stubImageRepo{})

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
	agg := metadata.NewAggregator([]ports.MetadataSource{src}, &stubImageRepo{})

	_, err := agg.FindByExternalID(context.Background(), domain.ContentTypeMusic, "some-id", "some-entity-id")
	if err == nil {
		t.Fatal("expected error when no sources match content type, got nil")
	}
}

func TestAggregator_FindByExternalID_PersistsImagesWithEntityInfo(t *testing.T) {
	const (
		externalID = "artist-mbid-123"
		entityID   = "internal-entry-uuid-abc"
	)
	src := &aggStubSource{
		name:         "fanart",
		contentTypes: []domain.ContentType{domain.ContentTypeMusic},
		findItem: &domain.ExternalItem{
			Source: "fanart",
			Images: []domain.ExternalImage{
				{Type: domain.ImageTypeBanner, URL: "https://example.com/banner.jpg"},
				{Type: domain.ImageTypeBackground, URL: "https://example.com/bg.jpg"},
			},
		},
	}
	imageRepo := &stubImageRepo{}
	agg := metadata.NewAggregator([]ports.MetadataSource{src}, imageRepo)

	_, err := agg.FindByExternalID(context.Background(), domain.ContentTypeMusic, externalID, entityID)
	if err != nil {
		t.Fatalf("FindByExternalID: %v", err)
	}

	stored, _ := imageRepo.List(context.Background(), "library_entry", entityID, nil)
	if len(stored) != 2 {
		t.Errorf("stored image count = %d, want 2", len(stored))
	}
	for _, img := range stored {
		if img.EntityType != "library_entry" {
			t.Errorf("EntityType = %q, want library_entry", img.EntityType)
		}
		if img.EntityID != entityID {
			t.Errorf("EntityID = %q, want %q (must be internal UUID, not external ID)", img.EntityID, entityID)
		}
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
	agg := metadata.NewAggregator([]ports.MetadataSource{src1, src2}, nil)

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
	agg := metadata.NewAggregator([]ports.MetadataSource{good, bad}, nil)

	items, err := agg.SearchItems(context.Background(), domain.ContentTypeMusic, "creep", 10)
	if err != nil {
		t.Fatalf("SearchItems should not return error when one source fails: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("item count = %d, want 1 (only from good source)", len(items))
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
	agg := metadata.NewAggregator([]ports.MetadataSource{src1, src2}, nil)

	items, err := agg.FetchEntryContent(context.Background(), domain.ContentTypeMusic, "radiohead-id")
	if err != nil {
		t.Fatalf("FetchEntryContent: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("item count = %d, want 3 (1 from mbz + 2 from fanart)", len(items))
	}
}
