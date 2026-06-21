//go:build integration

package lastfm_test

import (
	"context"
	"os"
	"testing"
	"time"

	"purser/internal/adapters/lastfm"
	"purser/internal/config"
)

func TestIntegration_SearchStudios_Radiohead(t *testing.T) {
	apiKey := os.Getenv("PURSER_SOURCES_LASTFM_API_KEY")
	if apiKey == "" {
		t.Skip("PURSER_SOURCES_LASTFM_API_KEY not set")
	}

	a := lastfm.New(config.MetadataSourceConfig{APIKey: apiKey})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := a.SearchStudios(ctx, "Radiohead", 10)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result, got 0")
	}

	var found bool
	for _, r := range results {
		if r.Name == "Radiohead" && r.ExternalID != "" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no result with Name=Radiohead and non-empty ExternalID; got: %+v", results)
	}
}
