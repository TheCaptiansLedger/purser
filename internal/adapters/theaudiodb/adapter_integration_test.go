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

const whitesnakeMBID = "5be36b67-c819-4337-bec6-b17e1cc9de70"

func integrationAdapter(t *testing.T) *theaudiodb.Adapter {
	t.Helper()
	apiKey := os.Getenv("PURSER_SOURCES_THEAUDIODB_API_KEY")
	if apiKey == "" {
		apiKey = "123" // free-tier key
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

func TestIntegration_FetchEntryContent_AlbumArt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, items, total, err := integrationAdapter(t).FetchEntryContent(ctx, domain.ContentTypeMusic, whitesnakeMBID, 1, 100)
	if err != nil {
		t.Fatalf("FetchEntryContent: %v", err)
	}
	if total == 0 || len(items) == 0 {
		t.Fatal("expected at least one album with cover art")
	}
	for _, item := range items {
		if item.ExternalIDs["mbid"] == "" {
			t.Errorf("item missing mbid in ExternalIDs: %+v", item)
		}
		if len(item.Images) == 0 || item.Images[0].URL == "" {
			t.Errorf("item missing cover image: mbid=%s", item.ExternalIDs["mbid"])
		}
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
}
