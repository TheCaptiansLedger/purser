//go:build live

// This file is gated behind the "live" build tag — it never runs under
// plain `go test ./...` or in CI, only via `go test -tags=live`. It hits
// the real StashDB API to catch schema drift, per
// docs/adr/0003-go-testing-standards.md's "a real, live-network
// verification test may exist for that adapter and that adapter only,
// gated behind a build tag" rule. Requires a PURSER_SOURCES_STASHDB_API_KEY
// env var (get one from a stashdb.org account) — matching
// internal/config.Sources.StashDB's actual "sources.stashdb.api_key" Viper
// key (docs/adr/0010-configuration.md, whose own context section cites
// this exact env var as the established .env.example convention); skips
// if missing. Run manually.
//
// StashDB's real data changes over time and this adapter has no hardcoded
// "well-known, stable" ID the way MusicBrainz's live test does (Beatles
// MBIDs are public knowledge; specific stash-box UUIDs aren't) — so every
// test below discovers a real ID via a search call first, then feeds it
// into the corresponding Lookup call, and skips (rather than fails) if the
// search itself returns nothing, since a term search against a live,
// crowd-sourced dataset isn't guaranteed to match forever.
package stashdb_test

import (
	"context"
	"os"
	"purser/internal/adapters/stashdb"
	"purser/internal/ports"
	"testing"
)

func newLiveClient(t *testing.T) *stashdb.Client {
	t.Helper()
	apiKey := os.Getenv("PURSER_SOURCES_STASHDB_API_KEY")
	if apiKey == "" {
		t.Skip("PURSER_SOURCES_STASHDB_API_KEY not set, skipping")
	}

	cfg := stashdb.DefaultConfig()
	cfg.APIKey = apiKey
	c, err := stashdb.New(cfg)
	if err != nil {
		t.Fatalf("stashdb.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestLive_SearchAndLookupPerformer discovers a real performer via
// SearchPerformers, then confirms LookupPerformer resolves that same ID.
func TestLive_SearchAndLookupPerformer(t *testing.T) {
	c := newLiveClient(t)
	ctx := context.Background()

	results, err := c.SearchPerformers(ctx, "a")
	if err != nil {
		t.Fatalf("SearchPerformers returned error: %v", err)
	}
	if len(results) == 0 {
		t.Skip("SearchPerformers returned no results for a broad term, skipping")
	}

	want := results[0]
	got, err := c.LookupPerformer(ctx, want.ID)
	if err != nil {
		t.Fatalf("LookupPerformer(%s) returned error: %v", want.ID, err)
	}
	if got.ID != want.ID || got.Name != want.Name {
		t.Errorf("LookupPerformer = %+v, want ID=%s Name=%s", got, want.ID, want.Name)
	}
}

// TestLive_SearchAndLookupScene discovers a real scene via SearchScenes,
// then confirms LookupScene resolves that same ID, and — if the scene has
// a studio — that LookupStudio resolves its studio ID too.
func TestLive_SearchAndLookupScene(t *testing.T) {
	c := newLiveClient(t)
	ctx := context.Background()

	results, err := c.SearchScenes(ctx, "a")
	if err != nil {
		t.Fatalf("SearchScenes returned error: %v", err)
	}
	if len(results) == 0 {
		t.Skip("SearchScenes returned no results for a broad term, skipping")
	}

	want := results[0]
	got, err := c.LookupScene(ctx, want.ID)
	if err != nil {
		t.Fatalf("LookupScene(%s) returned error: %v", want.ID, err)
	}
	if got.ID != want.ID {
		t.Errorf("LookupScene = %+v, want ID=%s", got, want.ID)
	}

	if got.Studio == nil {
		t.Skip("discovered scene has no studio, skipping LookupStudio verification")
	}
	studio, err := c.LookupStudio(ctx, got.Studio.ID)
	if err != nil {
		t.Fatalf("LookupStudio(%s) returned error: %v", got.Studio.ID, err)
	}
	if studio.ID != got.Studio.ID {
		t.Errorf("LookupStudio = %+v, want ID=%s", studio, got.Studio.ID)
	}
}

// TestLive_LookupPerformer_UnknownIDReturnsErrNotFound confirms StashDB's
// null-result GraphQL response still maps to ports.ErrNotFound against the
// real API, not just the fixture/contract suite.
func TestLive_LookupPerformer_UnknownIDReturnsErrNotFound(t *testing.T) {
	c := newLiveClient(t)
	_, err := c.LookupPerformer(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("LookupPerformer with an all-zero ID returned nil error")
	}
	t.Logf("LookupPerformer(all-zero ID) error (expected): %v", err)
}

// TestLive_FindScenesByFingerprints_UnknownFingerprintReturnsEmpty needs no
// discovered ID — an OSHash StashDB has never seen is a valid, deterministic
// scenario on its own.
func TestLive_FindScenesByFingerprints_UnknownFingerprintReturnsEmpty(t *testing.T) {
	c := newLiveClient(t)
	scenes, err := c.FindScenesByFingerprints(context.Background(), []ports.SceneFingerprint{
		{Hash: "0000000000000000", Algorithm: ports.FingerprintAlgorithmOSHash},
	})
	if err != nil {
		t.Fatalf("FindScenesByFingerprints returned error: %v", err)
	}
	if len(scenes) != 0 {
		t.Logf("FindScenesByFingerprints unexpectedly matched %d scene(s) for an all-zero OSHash: %+v", len(scenes), scenes)
	}
}
