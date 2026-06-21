//go:build integration

package stashdb_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"purser/internal/adapters/stashdb"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
)

func newIntegrationAdapter(t *testing.T) *stashdb.Adapter {
	t.Helper()
	apiKey := os.Getenv("PURSER_SOURCES_STASHDB_API_KEY")
	if apiKey == "" {
		t.Skip("PURSER_SOURCES_STASHDB_API_KEY not set")
	}
	return stashdb.New(config.MetadataSourceConfig{APIKey: apiKey})
}

func integrationCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// TestStashDB_SearchStudios verifies studio search returns results with required fields.
func TestStashDB_SearchStudios(t *testing.T) {
	a := newIntegrationAdapter(t)
	ctx, cancel := integrationCtx(t)
	defer cancel()

	results, err := a.SearchStudios(ctx, "Evil Angel", 5)
	if err != nil {
		t.Fatalf("SearchStudios: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchStudios returned no results for 'Evil Angel'")
	}
	first := results[0]
	if first.Name == "" {
		t.Error("results[0].Name is empty")
	}
	if first.ExternalID == "" {
		t.Error("results[0].ExternalID is empty")
	}
}

// TestStashDB_SearchPeople verifies performer search returns results with required fields.
func TestStashDB_SearchPeople(t *testing.T) {
	a := newIntegrationAdapter(t)
	ctx, cancel := integrationCtx(t)
	defer cancel()

	results, err := a.SearchPeople(ctx, "Mia Malkova", 5)
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchPeople returned no results for 'Mia Malkova'")
	}
	if results[0].Name == "" {
		t.Error("results[0].Name is empty")
	}
}

// TestStashDB_SearchItems verifies scene search returns results with required fields.
func TestStashDB_SearchItems(t *testing.T) {
	a := newIntegrationAdapter(t)
	ctx, cancel := integrationCtx(t)
	defer cancel()

	results, err := a.SearchItems(ctx, domain.ContentTypeAdult, "Summer", 5)
	if err != nil {
		t.Fatalf("SearchItems: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchItems returned no results for 'Summer'")
	}
	first := results[0]
	if first.Title == "" {
		t.Error("results[0].Title is empty")
	}
	if first.ContentType == "" {
		t.Error("results[0].ContentType is empty")
	}
}

// TestStashDB_FindByExternalID uses a scene ID obtained from SearchItems so the
// test stays resilient to platform churn while still exercising the findScene
// query and response mapping end-to-end.
func TestStashDB_FindByExternalID(t *testing.T) {
	a := newIntegrationAdapter(t)

	ctx1, cancel1 := integrationCtx(t)
	defer cancel1()

	results, err := a.SearchItems(ctx1, domain.ContentTypeAdult, "Summer", 1)
	if err != nil {
		t.Fatalf("SearchItems (setup): %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchItems returned no results; cannot run FindByExternalID")
	}
	sceneID := results[0].ExternalID

	ctx2, cancel2 := integrationCtx(t)
	defer cancel2()

	item, err := a.FindByExternalID(ctx2, domain.ContentTypeAdult, sceneID)
	if err != nil {
		t.Fatalf("FindByExternalID(%q): %v", sceneID, err)
	}
	if item.Title == "" {
		t.Error("FindByExternalID: Title is empty")
	}
	if item.ExternalID != sceneID {
		t.Errorf("ExternalID = %q, want %q", item.ExternalID, sceneID)
	}
}

// TestStashDB_FindByHash probes the fingerprint endpoint. OSHash coverage on
// StashDB is partial so ErrNotFound is an expected outcome; the test validates
// the transport and error-path only.
func TestStashDB_FindByHash(t *testing.T) {
	a := newIntegrationAdapter(t)
	ctx, cancel := integrationCtx(t)
	defer cancel()

	// Probe with an arbitrary 16-hex-char OSHash; ErrNotFound is acceptable.
	const probeHash = "da39a3ee5e6b4b0d"
	_, err := a.FindByHash(ctx, probeHash)
	if err != nil && !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("FindByHash: unexpected error: %v", err)
	}
}

// TestStashDB_FetchEntryContent fetches the first page of scenes for a studio
// obtained dynamically from SearchStudios. Validates the flat (no groups) layout
// and that total/items are populated.
func TestStashDB_FetchEntryContent(t *testing.T) {
	a := newIntegrationAdapter(t)

	ctx1, cancel1 := integrationCtx(t)
	defer cancel1()

	studios, err := a.SearchStudios(ctx1, "Evil Angel", 1)
	if err != nil {
		t.Fatalf("SearchStudios (setup): %v", err)
	}
	if len(studios) == 0 {
		t.Fatal("SearchStudios returned no results; cannot run FetchEntryContent")
	}
	studioID := studios[0].ExternalID

	ctx2, cancel2 := integrationCtx(t)
	defer cancel2()

	groups, items, total, err := a.FetchEntryContent(ctx2, domain.ContentTypeAdult, studioID, 1, 5)
	if err != nil {
		t.Fatalf("FetchEntryContent(%q): %v", studioID, err)
	}
	if groups != nil {
		t.Errorf("groups should be nil for StashDB flat hierarchy, got %d", len(groups))
	}
	if total == 0 {
		t.Error("FetchEntryContent: total is 0; studio should have scenes")
	}
	if len(items) == 0 {
		t.Error("FetchEntryContent: no items returned on page 1")
	}
}

// TestStashDB_FetchGroupContent confirms ErrNotSupported — StashDB has no
// intermediate group layer between a studio and its scenes.
func TestStashDB_FetchGroupContent(t *testing.T) {
	a := newIntegrationAdapter(t)
	ctx, cancel := integrationCtx(t)
	defer cancel()

	_, _, err := a.FetchGroupContent(ctx, domain.ContentTypeAdult, "any-id", 1, 10)
	if !errors.Is(err, ports.ErrNotSupported) {
		t.Errorf("FetchGroupContent: expected ErrNotSupported, got %v", err)
	}
}
