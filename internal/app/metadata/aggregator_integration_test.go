//go:build integration

package metadata_test

import (
	"context"
	"os"
	"purser/internal/adapters/fanart"
	"purser/internal/adapters/mbz"
	"purser/internal/app/metadata"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
	"time"
)

// radioheadMBID is Radiohead's canonical MusicBrainz artist identifier.
// Specified in issue #73 as the integration test fixture.
const radioheadMBID = "a74b1b7f-71a5-4011-9441-d0b5e4122711"

func TestMetadataAggregator_FindByExternalID_Music(t *testing.T) {
	apiKey := os.Getenv("PURSER_SOURCES_FANART_API_KEY")
	if apiKey == "" {
		t.Fatal("PURSER_SOURCES_FANART_API_KEY is required; set it to run aggregator integration tests")
	}

	sources := []ports.MetadataSource{
		mbz.New(config.MetadataSourceConfig{}),
		fanart.New(config.MetadataSourceConfig{APIKey: apiKey}),
	}
	agg := metadata.NewAggregator(sources)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const testEntityID = "test-entity-uuid-radiohead"
	item, err := agg.FindByExternalID(ctx, domain.ContentTypeMusic, radioheadMBID, testEntityID)
	if err != nil {
		t.Fatalf("FindByExternalID: %v", err)
	}

	if item.Title != "Radiohead" {
		t.Errorf("Title = %q, want Radiohead (primary source wins on title)", item.Title)
	}

	if item.ExternalIDs["mbid"] == "" {
		t.Error("ExternalIDs[mbid] is empty; MusicBrainz primary must populate this field")
	}

	hasFanartImage := false
	for _, img := range item.Images {
		if img.Source == "fanart" {
			hasFanartImage = true
			break
		}
	}
	if !hasFanartImage {
		t.Error("Images contains no entry with Source == fanart")
	}

	imageTypes := map[domain.ImageType]bool{}
	for _, img := range item.Images {
		imageTypes[img.Type] = true
	}
	if len(imageTypes) < 2 {
		t.Errorf("Images contain only %d distinct ImageType value(s); want >= 2", len(imageTypes))
	}
}
