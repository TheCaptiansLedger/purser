//go:build live

// This file is gated behind the "live" build tag — it never runs under
// plain `go test ./...` or in CI, only via `go test -tags=live`. It hits
// the real MusicBrainz API to catch schema drift, per
// docs/adr/0003-go-testing-standards.md's "a real, live-network
// verification test may exist for that adapter and that adapter only,
// gated behind a build tag" rule and issue #509's verification checklist.
// Run manually; MusicBrainz's own 1 req/sec limit means this is slow by
// design (each of these creates its own Client, so the rate limiter
// doesn't serialize across them, but MusicBrainz's server-side policy
// still applies per real request).
package musicbrainz_test

import (
	"context"
	"purser/internal/adapters/musicbrainz"
	"testing"
)

func newLiveClient(t *testing.T) *musicbrainz.Client {
	t.Helper()
	c, err := musicbrainz.New(musicbrainz.DefaultConfig())
	if err != nil {
		t.Fatalf("musicbrainz.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestLive_LookupArtist hits the real MusicBrainz API for a well-known,
// stable MBID (The Beatles) and checks the response shape this adapter
// assumes still holds.
func TestLive_LookupArtist(t *testing.T) {
	c := newLiveClient(t)
	a, err := c.LookupArtist(context.Background(), "b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d")
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}
	if a.Name != "The Beatles" {
		t.Errorf("Name = %q, want %q", a.Name, "The Beatles")
	}
	if a.Type != "Group" {
		t.Errorf("Type = %q, want %q", a.Type, "Group")
	}
}

// TestLive_SearchReleaseByBarcode hits the real MusicBrainz API for a
// well-known barcode (the 2009 US remaster of Please Please Me).
func TestLive_SearchReleaseByBarcode(t *testing.T) {
	c := newLiveClient(t)
	releases, err := c.SearchReleaseByBarcode(context.Background(), "094638241621")
	if err != nil {
		t.Fatalf("SearchReleaseByBarcode returned error: %v", err)
	}
	if len(releases) == 0 {
		t.Fatal("SearchReleaseByBarcode returned no releases")
	}
	if releases[0].Barcode != "094638241621" {
		t.Errorf("Barcode = %q, want 094638241621", releases[0].Barcode)
	}
}

// TestLive_LookupRecordingByISRC hits the real MusicBrainz API for a
// well-known ISRC (a Please Please Me track).
func TestLive_LookupRecordingByISRC(t *testing.T) {
	c := newLiveClient(t)
	recs, err := c.LookupRecordingByISRC(context.Background(), "GBAYE0600350")
	if err != nil {
		t.Fatalf("LookupRecordingByISRC returned error: %v", err)
	}
	if len(recs) == 0 {
		t.Fatal("LookupRecordingByISRC returned no recordings")
	}
	if recs[0].Title != "Please Please Me" {
		t.Errorf("Title = %q, want %q", recs[0].Title, "Please Please Me")
	}
}
