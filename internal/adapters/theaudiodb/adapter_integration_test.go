//go:build integration

package theaudiodb_test

import (
	"context"
	"os"
	"purser/internal/adapters/theaudiodb"
	"purser/internal/config"
	"purser/internal/domain"
	"testing"
	"time"
)

const whitesnakeMBID = "5dedf5cf-a598-4408-9556-3bf3f149f3ba"

// Come an' Get It — confirmed to have strAlbumThumb in TheAudioDB
const whitesnakeAlbumMBID = "2d4dfb06-6a4f-311c-88e7-4a7eae7e08f6"

func integrationAdapter(t *testing.T) *theaudiodb.Adapter {
	t.Helper()
	apiKey := os.Getenv("PURSER_SOURCES_THEAUDIODB_API_KEY")
	if apiKey == "" {
		t.Skip("PURSER_SOURCES_THEAUDIODB_API_KEY not set (use 123 for free tier)")
	}
	return theaudiodb.New(config.MetadataSourceConfig{APIKey: apiKey})
}

func TestIntegration_FindByExternalID_Artist(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	item, err := integrationAdapter(t).FindByExternalID(ctx, domain.ContentTypeMusic, whitesnakeMBID)
	if err != nil {
		t.Fatalf("FindByExternalID: %v", err)
	}
	if item.Title == "" {
		t.Error("Title should be non-empty")
	}
	if item.ExternalID == "" {
		t.Error("ExternalID should be non-empty")
	}
	if item.ImageURL == "" {
		t.Error("ImageURL (strArtistThumb) should be non-empty")
	}
	if len(item.Images) == 0 {
		t.Error("Images should be non-empty")
	}
}

func TestIntegration_FetchGroupContent_AlbumCover(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	items, total, err := integrationAdapter(t).FetchGroupContent(ctx, domain.ContentTypeMusic, whitesnakeAlbumMBID, 1, 1)
	if err != nil {
		t.Fatalf("FetchGroupContent: %v", err)
	}
	if total == 0 || len(items) == 0 {
		t.Fatal("expected album cover art for Come an' Get It")
	}
	if len(items[0].Images) == 0 || items[0].Images[0].URL == "" {
		t.Error("ImageURL (strAlbumThumb) should be non-empty")
	}
	if items[0].ExternalIDs["mbid"] == "" {
		t.Error("ExternalIDs[mbid] should be non-empty")
	}
}

func TestIntegration_SearchStudios(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := integrationAdapter(t).SearchStudios(ctx, "Whitesnake", 5)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Name == "" {
		t.Error("Name should be non-empty")
	}
	if results[0].ExternalID == "" {
		t.Error("ExternalID should be non-empty")
	}
	if results[0].ImageURL == "" {
		t.Error("ImageURL should be non-empty for Whitesnake")
	}
}
