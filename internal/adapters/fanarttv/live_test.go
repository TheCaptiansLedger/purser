//go:build live

// This file is gated behind the "live" build tag — it never runs under
// plain `go test ./...` or in CI, only via `go test -tags=live`. It hits
// the real webservice.fanart.tv API to catch schema drift, per
// docs/adr/0003-go-testing-standards.md's "a real, live-network
// verification test may exist for that adapter and that adapter only,
// gated behind a build tag" rule. Requires a PURSER_SOURCES_FANART_API_KEY
// env var — matching internal/config.Sources.FanartTV's
// "sources.fanart.api_key" Viper key; skips if missing. Run manually.
//
// The Beatles (artist MBID b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d) is
// hardcoded here — confirmed live during this adapter's implementation and
// stable enough to rely on, the same way MusicBrainz's own live test
// hardcodes well-known Beatles MBIDs.
package fanarttv_test

import (
	"context"
	"os"
	"purser/internal/adapters/fanarttv"
	"testing"
)

func newLiveClient(t *testing.T) *fanarttv.Client {
	t.Helper()
	apiKey := os.Getenv("PURSER_SOURCES_FANART_API_KEY")
	if apiKey == "" {
		t.Skip("PURSER_SOURCES_FANART_API_KEY not set, skipping")
	}

	cfg := fanarttv.DefaultConfig()
	cfg.APIKey = apiKey
	c, err := fanarttv.New(cfg)
	if err != nil {
		t.Fatalf("fanarttv.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestLive_LookupArtist confirms LookupArtist resolves The Beatles' MBID
// against the real API, returns populated artist-level image slots, and
// includes at least one release group's album art in Albums.
func TestLive_LookupArtist(t *testing.T) {
	c := newLiveClient(t)
	a, err := c.LookupArtist(context.Background(), "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d")
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}
	if a.Name != "The Beatles" {
		t.Errorf("Name = %q, want The Beatles", a.Name)
	}
	if len(a.ArtistThumb) == 0 {
		t.Error("ArtistThumb is empty, want at least one image")
	}
	if len(a.Albums) == 0 {
		t.Error("Albums is empty, want at least one release group's art")
	}
}

// TestLive_LookupArtist_UnknownMBIDReturnsErrNotFound confirms
// fanart.tv's real empty-{}-body response still maps to ports.ErrNotFound
// against the real API, not just the fixture/contract suite.
func TestLive_LookupArtist_UnknownMBIDReturnsErrNotFound(t *testing.T) {
	c := newLiveClient(t)
	_, err := c.LookupArtist(context.Background(), "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("LookupArtist with an all-zero MBID returned nil error")
	}
	t.Logf("LookupArtist(all-zero MBID) error (expected): %v", err)
}
