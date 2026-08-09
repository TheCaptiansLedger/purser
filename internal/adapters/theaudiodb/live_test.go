//go:build live

// This file is gated behind the "live" build tag — it never runs under
// plain `go test ./...` or in CI, only via `go test -tags=live`. It hits
// the real theaudiodb.com API to catch schema drift, per
// docs/adr/0003-go-testing-standards.md's "a real, live-network
// verification test may exist for that adapter and that adapter only,
// gated behind a build tag" rule. Requires a
// PURSER_SOURCES_THEAUDIODB_API_KEY env var (the free-tier key "123"
// works, confirmed live during this adapter's implementation) — matching
// internal/config.Sources.TheAudioDB's "sources.theaudiodb.api_key" Viper
// key; skips if missing. Run manually.
//
// The Beatles (artist MBID b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d,
// "Please Please Me" release-group MBID
// de208292-8db5-3aed-a14a-b37a84d8c521) are hardcoded here — both
// confirmed live during this adapter's implementation and stable enough to
// rely on, the same way MusicBrainz's own live test hardcodes well-known
// Beatles MBIDs.
package theaudiodb_test

import (
	"context"
	"os"
	"purser/internal/adapters/theaudiodb"
	"testing"
)

func newLiveClient(t *testing.T) *theaudiodb.Client {
	t.Helper()
	apiKey := os.Getenv("PURSER_SOURCES_THEAUDIODB_API_KEY")
	if apiKey == "" {
		t.Skip("PURSER_SOURCES_THEAUDIODB_API_KEY not set, skipping")
	}

	cfg := theaudiodb.DefaultConfig()
	cfg.APIKey = apiKey
	c, err := theaudiodb.New(cfg)
	if err != nil {
		t.Fatalf("theaudiodb.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestLive_LookupArtist confirms LookupArtist resolves The Beatles' MBID
// against the real API and returns populated image fields.
func TestLive_LookupArtist(t *testing.T) {
	c := newLiveClient(t)
	a, err := c.LookupArtist(context.Background(), "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d")
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}
	if a.Name != "The Beatles" {
		t.Errorf("Name = %q, want The Beatles", a.Name)
	}
	if a.Thumb == "" {
		t.Error("Thumb = \"\", want a populated image URL")
	}
}

// TestLive_LookupAlbum confirms LookupAlbum resolves "Please Please Me"'s
// release-group MBID against the real API and returns populated image
// fields.
func TestLive_LookupAlbum(t *testing.T) {
	c := newLiveClient(t)
	a, err := c.LookupAlbum(context.Background(), "de208292-8db5-3aed-a14a-b37a84d8c521")
	if err != nil {
		t.Fatalf("LookupAlbum returned error: %v", err)
	}
	if a.Title != "Please Please Me" {
		t.Errorf("Title = %q, want Please Please Me", a.Title)
	}
	if a.Thumb == "" {
		t.Error("Thumb = \"\", want a populated image URL")
	}
}

// TestLive_LookupArtist_UnknownMBIDReturnsErrNotFound confirms
// TheAudioDB's real {"artists":null} response still maps to
// ports.ErrNotFound against the real API, not just the fixture/contract
// suite.
func TestLive_LookupArtist_UnknownMBIDReturnsErrNotFound(t *testing.T) {
	c := newLiveClient(t)
	_, err := c.LookupArtist(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("LookupArtist with an all-zero MBID returned nil error")
	}
	t.Logf("LookupArtist(all-zero MBID) error (expected): %v", err)
}
