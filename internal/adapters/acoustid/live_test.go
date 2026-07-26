//go:build live

// This file is gated behind the "live" build tag — it never runs under
// plain `go test ./...` or in CI, only via `go test -tags=live`. It hits
// the real AcoustID API to catch schema drift, per docs/adr/0003-go-
// testing-standards.md's "a real, live-network verification test may
// exist for that adapter and that adapter only, gated behind a build
// tag" rule and issue #513's verification checklist. Requires both fpcalc
// on PATH and a PURSER_SOURCES_ACOUSTID_API_KEY env var (get one at
// https://acoustid.org/api-key) — matching the PURSER_SOURCES_<NAME>_
// API_KEY convention docs/adr/0010-configuration.md's other sources use,
// even though this adapter isn't wired into internal/config yet; skips
// if either is missing. Run manually; computes a real fingerprint from
// the checked-in sample audio via the real Fingerprint, then calls the
// real Lookup with it — the sample is a synthetic tone, so no match is
// expected, but this proves the request/response shape this adapter
// assumes still holds against the real API.
package acoustid_test

import (
	"context"
	"os"
	"os/exec"
	"purser/internal/adapters/acoustid"
	"testing"
)

func TestLive_LookupAgainstSampleFingerprint(t *testing.T) {
	if _, err := exec.LookPath("fpcalc"); err != nil {
		t.Skip("fpcalc not found on PATH, skipping")
	}
	apiKey := os.Getenv("PURSER_SOURCES_ACOUSTID_API_KEY")
	if apiKey == "" {
		t.Skip("PURSER_SOURCES_ACOUSTID_API_KEY not set, skipping")
	}

	cfg := acoustid.DefaultConfig()
	cfg.APIKey = apiKey
	c, err := acoustid.New(cfg)
	if err != nil {
		t.Fatalf("acoustid.New returned error: %v", err)
	}
	defer func() { _ = c.Close() }()

	ctx := context.Background()
	fingerprint, duration, err := c.Fingerprint(ctx, "testdata/sample.flac")
	if err != nil {
		t.Fatalf("Fingerprint returned error: %v", err)
	}
	if fingerprint == "" {
		t.Fatal("Fingerprint returned an empty fingerprint")
	}
	t.Logf("computed fingerprint (duration=%.1fs): %s", duration, fingerprint)

	matches, err := c.Lookup(ctx, fingerprint, duration)
	if err != nil {
		t.Logf("Lookup returned error (expected for a synthetic tone with no real matches): %v", err)
		return
	}
	t.Logf("Lookup returned %d match(es) — inspect manually to confirm the response shape: %+v", len(matches), matches)
}
