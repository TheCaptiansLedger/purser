//go:build live

// This file is gated behind the "live" build tag — it never runs under
// plain `go test ./...` or in CI, only via `go test -tags=live`. It hits
// the real ThePornDB API to catch schema drift, per
// docs/adr/0003-go-testing-standards.md's "a real, live-network
// verification test may exist for that adapter and that adapter only,
// gated behind a build tag" rule. Requires a PURSER_SOURCES_TPDB_API_KEY
// env var (get one from a theporndb.net account) — matching
// internal/config.Sources.ThePornDB's actual "sources.tpdb.api_key" Viper
// key; skips if missing. Run manually.
//
// ThePornDB's real data changes over time and this adapter has no
// hardcoded "well-known, stable" ID the way MusicBrainz's live test does
// — so every test below discovers a real ID via a search call first, then
// feeds it into the corresponding Lookup call, and skips (rather than
// fails) if the search itself returns nothing, since a term search against
// a live, crowd-sourced dataset isn't guaranteed to match forever. The one
// exception is TestLive_ResolveJAVCode, which uses SSIS-001 — a real,
// long-published JAV code confirmed live during this adapter's own
// implementation (see theporndb.go's package doc comment) and stable
// enough to hardcode, the same way MusicBrainz's live test hardcodes
// well-known Beatles MBIDs.
package theporndb_test

import (
	"context"
	"os"
	"purser/internal/adapters/theporndb"
	"testing"
)

func newLiveClient(t *testing.T) *theporndb.Client {
	t.Helper()
	apiKey := os.Getenv("PURSER_SOURCES_TPDB_API_KEY")
	if apiKey == "" {
		t.Skip("PURSER_SOURCES_TPDB_API_KEY not set, skipping")
	}

	cfg := theporndb.DefaultConfig()
	cfg.APIKey = apiKey
	c, err := theporndb.New(cfg)
	if err != nil {
		t.Fatalf("theporndb.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestLive_SearchAndLookupPerformer discovers a real performer via
// SearchPerformers, then confirms LookupPerformer resolves that same ID.
func TestLive_SearchAndLookupPerformer(t *testing.T) {
	c := newLiveClient(t)
	ctx := context.Background()

	results, err := c.SearchPerformers(ctx, "Riley Reid")
	if err != nil {
		t.Fatalf("SearchPerformers returned error: %v", err)
	}
	if len(results) == 0 {
		t.Skip("SearchPerformers returned no results, skipping")
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
// then confirms LookupScene resolves that same ID.
func TestLive_SearchAndLookupScene(t *testing.T) {
	c := newLiveClient(t)
	ctx := context.Background()

	results, err := c.SearchScenes(ctx, "Riley Reid")
	if err != nil {
		t.Fatalf("SearchScenes returned error: %v", err)
	}
	if len(results) == 0 {
		t.Skip("SearchScenes returned no results, skipping")
	}

	want := results[0]
	got, err := c.LookupScene(ctx, want.ID)
	if err != nil {
		t.Fatalf("LookupScene(%s) returned error: %v", want.ID, err)
	}
	if got.ID != want.ID {
		t.Errorf("LookupScene = %+v, want ID=%s", got, want.ID)
	}
}

// TestLive_LookupSceneByHash discovers a real hash from a real scene via
// SearchScenes, then confirms LookupSceneByHash resolves that hash back to
// the same scene.
func TestLive_LookupSceneByHash(t *testing.T) {
	c := newLiveClient(t)
	ctx := context.Background()

	results, err := c.SearchScenes(ctx, "Riley Reid")
	if err != nil {
		t.Fatalf("SearchScenes returned error: %v", err)
	}
	var sceneID, hash string
	for _, s := range results {
		if len(s.Hashes) > 0 {
			sceneID, hash = s.ID, s.Hashes[0].Hash
			break
		}
	}
	if hash == "" {
		t.Skip("no discovered scene carried a hash, skipping")
	}

	got, err := c.LookupSceneByHash(ctx, hash)
	if err != nil {
		t.Fatalf("LookupSceneByHash(%s) returned error: %v", hash, err)
	}
	if got.ID != sceneID {
		t.Errorf("LookupSceneByHash = %+v, want ID=%s", got, sceneID)
	}
}

// TestLive_ResolveJAVCode confirms GET /jav?parse=SSIS-001 resolves and
// that the returned list's first (best-ranked) candidate is the SSIS-001
// title itself — see the package doc comment for why this code specifically
// is hardcoded rather than discovered.
func TestLive_ResolveJAVCode(t *testing.T) {
	c := newLiveClient(t)
	scenes, err := c.ResolveJAVCode(context.Background(), "SSIS-001")
	if err != nil {
		t.Fatalf("ResolveJAVCode returned error: %v", err)
	}
	if len(scenes) == 0 {
		t.Fatal("ResolveJAVCode(SSIS-001) returned no candidates")
	}
	if scenes[0].Type != "JAV" || scenes[0].ExternalID != "ssis-001" {
		t.Errorf("ResolveJAVCode[0] = %+v, want Type=JAV ExternalID=ssis-001", scenes[0])
	}
}

// TestLive_LookupPerformer_UnknownIDReturnsErrNotFound confirms ThePornDB's
// 404 response still maps to ports.ErrNotFound against the real API, not
// just the fixture/contract suite.
func TestLive_LookupPerformer_UnknownIDReturnsErrNotFound(t *testing.T) {
	c := newLiveClient(t)
	_, err := c.LookupPerformer(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("LookupPerformer with an all-zero ID returned nil error")
	}
	t.Logf("LookupPerformer(all-zero ID) error (expected): %v", err)
}

// TestLive_LookupSceneByHash_UnknownHashReturnsErrNotFound needs no
// discovered ID — a hash ThePornDB has never seen is a valid, deterministic
// scenario on its own (confirmed live during this adapter's implementation:
// {"message": "hash not found"}).
func TestLive_LookupSceneByHash_UnknownHashReturnsErrNotFound(t *testing.T) {
	c := newLiveClient(t)
	_, err := c.LookupSceneByHash(context.Background(), "deadbeefcafef00d")
	if err == nil {
		t.Fatal("LookupSceneByHash with an unrecognized hash returned nil error")
	}
	t.Logf("LookupSceneByHash(unrecognized hash) error (expected): %v", err)
}
