//go:build integration

package fanart_test

import (
	"context"
	"errors"
	"os"
	"purser/internal/adapters/fanart"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"testing"
	"time"
)

// radioheadMBID is Radiohead's canonical MusicBrainz artist identifier.
// Stable and specified in issue #74 as the integration test fixture.
const radioheadMBID = "a74b1b7f-71a5-4011-9441-d0b5e4122711"

func newIntegrationAdapter(t *testing.T) *fanart.Adapter {
	t.Helper()
	apiKey := os.Getenv("PURSER_SOURCES_FANART_API_KEY")
	if apiKey == "" {
		t.Skip("PURSER_SOURCES_FANART_API_KEY not set")
	}
	return fanart.New(config.MetadataSourceConfig{APIKey: apiKey})
}

func integrationCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func TestFanart_FindByExternalID_Music(t *testing.T) {
	a := newIntegrationAdapter(t)
	ctx, cancel := integrationCtx(t)
	defer cancel()

	item, err := a.FindByExternalID(ctx, domain.ContentTypeMusic, radioheadMBID)
	if err != nil {
		t.Fatalf("FindByExternalID: %v", err)
	}
	if item == nil {
		t.Fatal("FindByExternalID returned nil item")
	}
	if len(item.Images) == 0 {
		t.Fatal("Images is empty")
	}

	hasThumbnailOrHero := false
	hasBanner := false
	knownTypes := map[domain.ImageType]bool{
		domain.ImageTypeThumbnail:  true,
		domain.ImageTypeBackground: true,
		domain.ImageTypeBanner:     true,
	}

	for _, img := range item.Images {
		if img.URL == "" {
			t.Errorf("image of type %q has empty URL", img.Type)
		}
		if !strings.HasPrefix(img.URL, "https://") {
			t.Errorf("image URL %q does not begin with https://", img.URL)
		}
		if !knownTypes[img.Type] {
			t.Errorf("image has unmapped/unknown type %q", img.Type)
		}
		if img.Type == domain.ImageTypeThumbnail || img.Type == domain.ImageTypeHero {
			hasThumbnailOrHero = true
		}
		if img.Type == domain.ImageTypeBanner {
			hasBanner = true
		}
	}

	if !hasThumbnailOrHero {
		t.Error("expected at least one image with type thumbnail or hero")
	}
	if !hasBanner {
		t.Error("expected at least one image with type banner")
	}
}

func TestFanart_FetchEntryContent_Music(t *testing.T) {
	a := newIntegrationAdapter(t)
	ctx, cancel := integrationCtx(t)
	defer cancel()

	_, items, total, err := a.FetchEntryContent(ctx, domain.ContentTypeMusic, radioheadMBID, 1, 100)
	if err != nil {
		t.Fatalf("FetchEntryContent: %v", err)
	}
	if total == 0 {
		t.Fatal("total = 0; Radiohead must have albums with artwork in fanart.tv")
	}
	if len(items) == 0 {
		t.Fatal("items is empty on page 1")
	}

	for i, item := range items {
		mbid := item.ExternalIDs["mbid"]
		if mbid == "" {
			t.Errorf("items[%d].ExternalIDs[mbid] is empty", i)
		}
		hasPoster := false
		for _, img := range item.Images {
			if img.Type == domain.ImageTypePoster {
				hasPoster = true
			}
		}
		if !hasPoster {
			t.Errorf("items[%d] (mbid=%q) has no poster image", i, mbid)
		}
	}
}

func TestFanart_SearchByTitle_NotSupported(t *testing.T) {
	a := newIntegrationAdapter(t)
	ctx, cancel := integrationCtx(t)
	defer cancel()

	_, err := a.SearchStudios(ctx, "Radiohead", 10)
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("expected ErrNotSupported from SearchStudios, got: %v", err)
	}
}
