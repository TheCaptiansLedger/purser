package metadata_test

import (
	"context"
	"purser/internal/app/metadata"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// newServiceWithSources builds a Service wired to the given metadata sources.
func newServiceWithSources(sources ...ports.MetadataSource) *metadata.Service {
	return metadata.New(
		sources,
		nil,
		newStubEntryRepo(),
		&stubGroupRepo{},
		&stubItemRepo{},
		&stubPersonRepo{},
		&stubTagRepo{},
		&stubExternalIDRepo{},
		nil,
	)
}

// seedEntry imports a library entry with the given external ID via ImportStudio,
// returning the assigned internal ID.
func seedEntry(t *testing.T, svc *metadata.Service, source domain.ExternalIDSource, extID string) string {
	t.Helper()
	res, err := svc.ImportStudio(context.Background(), &metadata.ImportStudioRequest{
		Source:      source,
		ExternalID:  extID,
		Name:        "Test Studio",
		ContentType: domain.ContentTypeAdult,
	})
	if err != nil {
		t.Fatalf("seed entry: %v", err)
	}
	return res.Studio.ID
}

// seedPerson imports a person with the given external ID via ImportPerson,
// returning the assigned internal ID.
func seedPerson(t *testing.T, svc *metadata.Service, source domain.ExternalIDSource, extID string) string {
	t.Helper()
	p, err := svc.ImportPerson(context.Background(), &metadata.ImportPersonRequest{
		Source:     source,
		ExternalID: extID,
		Name:       "Test Person",
	})
	if err != nil {
		t.Fatalf("seed person: %v", err)
	}
	return p.ID
}

// ── FetchImagesForEntry ───────────────────────────────────────────────────────

func TestFetchImagesForEntry_NotFound(t *testing.T) {
	svc := newServiceWithSources()
	_, err := svc.FetchImagesForEntry(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown entry ID, got nil")
	}
}

func TestFetchImagesForEntry_SourceReturnsNoImages(t *testing.T) {
	src := &stubImageSource{
		sourceName:   "stashdb",
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		findErr:      ports.ErrNotFound,
	}
	svc := newServiceWithSources(src)
	entryID := seedEntry(t, svc, domain.SourceStashDB, "ext-1")

	images, err := svc.FetchImagesForEntry(context.Background(), entryID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}

func TestFetchImagesForEntry_ReturnsImages(t *testing.T) {
	src := &stubImageSource{
		sourceName:   "stashdb",
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		findItem: &domain.ExternalItem{
			Images: []domain.ExternalImage{
				{Type: domain.ImageTypePoster, URL: "https://example.com/poster.jpg", Width: 640, Height: 480},
				{Type: domain.ImageTypeHero, URL: "https://example.com/hero.jpg"},
			},
		},
	}
	svc := newServiceWithSources(src)
	entryID := seedEntry(t, svc, domain.SourceStashDB, "studio-abc")

	images, err := svc.FetchImagesForEntry(context.Background(), entryID)
	if err != nil {
		t.Fatalf("FetchImagesForEntry: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("got %d images, want 2", len(images))
	}
	if images[0].URL != "https://example.com/poster.jpg" {
		t.Errorf("image[0].URL = %q, want poster URL", images[0].URL)
	}
	if images[1].URL != "https://example.com/hero.jpg" {
		t.Errorf("image[1].URL = %q, want hero URL", images[1].URL)
	}
}

func TestFetchImagesForEntry_DeduplicatesByURL(t *testing.T) {
	sharedURL := "https://example.com/shared.jpg"
	src := &stubImageSource{
		sourceName:   "stashdb",
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		findItem: &domain.ExternalItem{
			Images: []domain.ExternalImage{
				{Type: domain.ImageTypePoster, URL: sharedURL},
				{Type: domain.ImageTypePoster, URL: sharedURL}, // duplicate
				{Type: domain.ImageTypeHero, URL: "https://example.com/unique.jpg"},
			},
		},
	}
	svc := newServiceWithSources(src)
	entryID := seedEntry(t, svc, domain.SourceStashDB, "studio-dedup")

	images, err := svc.FetchImagesForEntry(context.Background(), entryID)
	if err != nil {
		t.Fatalf("FetchImagesForEntry: %v", err)
	}
	if len(images) != 2 {
		t.Errorf("got %d images, want 2 (duplicate URL should be removed)", len(images))
	}
}

// ── FetchImagesForGroup ───────────────────────────────────────────────────────

func TestFetchImagesForGroup_NotFound(t *testing.T) {
	svc := newServiceWithSources()
	_, err := svc.FetchImagesForGroup(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown group ID, got nil")
	}
}

func TestFetchImagesForGroup_ReturnsImages(t *testing.T) {
	// stubMusicSource handles FetchGroupContent (returns empty tracks) and
	// FindByExternalID (returns the configured item with images).
	src := &stubMusicSource{
		findItem: &domain.ExternalItem{
			Images: []domain.ExternalImage{
				{Type: domain.ImageTypePoster, URL: "https://example.com/album.jpg"},
			},
		},
		tracks: map[string][]*domain.ExternalItem{},
	}
	svc := newServiceWithSources(src)

	entryRes, err := svc.ImportStudio(context.Background(), &metadata.ImportStudioRequest{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  "artist-mbid",
		Name:        "Test Artist",
		ContentType: domain.ContentTypeMusic,
	})
	if err != nil {
		t.Fatalf("seed entry: %v", err)
	}
	groupRes, err := svc.ImportAlbum(context.Background(), &metadata.ImportAlbumRequest{
		Source:         domain.SourceMusicBrainz,
		ExternalID:     "album-mbid",
		LibraryEntryID: entryRes.Studio.ID,
		Title:          "Test Album",
	})
	if err != nil {
		t.Fatalf("seed group: %v", err)
	}

	images, err := svc.FetchImagesForGroup(context.Background(), groupRes.ID)
	if err != nil {
		t.Fatalf("FetchImagesForGroup: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("got %d images, want 1", len(images))
	}
	if images[0].URL != "https://example.com/album.jpg" {
		t.Errorf("image URL = %q, want album.jpg", images[0].URL)
	}
}

// ── FetchImagesForItem ────────────────────────────────────────────────────────

func TestFetchImagesForItem_NotFound(t *testing.T) {
	svc := newServiceWithSources()
	_, err := svc.FetchImagesForItem(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown item ID, got nil")
	}
}

// ── FetchImagesForPerson ──────────────────────────────────────────────────────

func TestFetchImagesForPerson_NotFound(t *testing.T) {
	svc := newServiceWithSources()
	_, err := svc.FetchImagesForPerson(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown person ID, got nil")
	}
}

func TestFetchImagesForPerson_ReturnsImages(t *testing.T) {
	src := &stubImageSource{
		sourceName:   "stashdb",
		contentTypes: []domain.ContentType{domain.ContentTypeAdult},
		findItem: &domain.ExternalItem{
			Images: []domain.ExternalImage{
				{Type: domain.ImageTypeHero, URL: "https://example.com/performer.jpg"},
			},
		},
	}
	svc := newServiceWithSources(src)
	personID := seedPerson(t, svc, domain.SourceStashDB, "performer-abc")

	images, err := svc.FetchImagesForPerson(context.Background(), personID)
	if err != nil {
		t.Fatalf("FetchImagesForPerson: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("got %d images, want 1", len(images))
	}
	if images[0].URL != "https://example.com/performer.jpg" {
		t.Errorf("image URL = %q, want performer.jpg", images[0].URL)
	}
}
