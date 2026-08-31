//go:build live

// This file is gated behind the "live" build tag — it never runs under
// plain `go test ./...` or in CI, only via `go test -tags=live`. It hits
// the real www.wikidata.org action API to catch schema drift, per
// docs/adr/0003-go-testing-standards.md's "a real, live-network
// verification test may exist for that adapter and that adapter only,
// gated behind a build tag" rule. No API key needed — unlike
// fanarttv/theaudiodb/stashdb/theporndb's live tests, Wikidata's action
// API requires no authentication, so this always runs when invoked (no
// skip-if-missing-env-var guard).
//
// REO Speedwagon (Q845084) is hardcoded here — confirmed live during this
// adapter's implementation to carry two P18 image claims, and stable
// enough to rely on, same precedent fanarttv's live test sets with The
// Beatles.
package wikidata_test

import (
	"context"
	"errors"
	"purser/internal/adapters/wikidata"
	"purser/internal/ports"
	"testing"
)

func newLiveClient(t *testing.T) *wikidata.Client {
	t.Helper()
	c, err := wikidata.New(wikidata.DefaultConfig())
	if err != nil {
		t.Fatalf("wikidata.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// TestLive_LookupImage confirms LookupImage resolves REO Speedwagon's
// Wikidata entity against the real API and returns at least one
// hotlinkable Commons file URL.
func TestLive_LookupImage(t *testing.T) {
	c := newLiveClient(t)
	images, err := c.LookupImage(context.Background(), "https://www.wikidata.org/wiki/Q845084")
	if err != nil {
		t.Fatalf("LookupImage returned error: %v", err)
	}
	if len(images) == 0 {
		t.Fatal("LookupImage returned no images for Q845084, want at least one")
	}
	for _, img := range images {
		if img.URL == "" {
			t.Errorf("images = %+v, want every entry to have a non-empty URL", images)
		}
	}
}

// TestLive_LookupImage_NoSuchEntity confirms a syntactically valid but
// nonexistent QID maps to ports.ErrNotFound against the real API.
func TestLive_LookupImage_NoSuchEntity(t *testing.T) {
	c := newLiveClient(t)
	_, err := c.LookupImage(context.Background(), "https://www.wikidata.org/wiki/Q999999999")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("LookupImage error = %v, want wrapping ports.ErrNotFound", err)
	}
}
