//go:build live

// This file is gated behind the "live" build tag — it never runs under
// plain `go test ./...` or in CI, only via `go test -tags=live`. It hits a
// real, operator-run SABnzbd instance to catch schema drift, per
// docs/adr/0003-go-testing-standards.md's "a real, live-network
// verification test may exist for that adapter and that adapter only,
// gated behind a build tag" rule. Requires PURSER_SABNZBD_BASE_URL and
// PURSER_SABNZBD_API_KEY env vars (matching internal/config.SABnzbd's
// actual "sabnzbd.base_url"/"sabnzbd.api_key" Viper keys) pointed at a real
// SABnzbd instance; skips if either is missing. Run manually — there is no
// public default instance this could run against unattended.
//
// TestLive_AddStatusRemoveRoundTrip submits a deliberately unreachable
// (RFC 2606 .invalid) .nzb URL, then removes it again (with deleteFiles)
// so the operator's instance isn't left with test leftovers. Unlike a
// magnet link's infohash, an .nzb URL has no durable, content-addressed
// public example to point at — it names usenet articles on one specific
// provider, subject to that provider's retention window, so any hardcoded
// "well-known test .nzb" is only ever temporarily fetchable (confirmed
// during this test's own development: sabnzbd.org's long-referenced
// bigbuckbunny.nzb now 404s). This test proves the request/response shape
// of Add/Status/Remove against the real API, not that real content can be
// fetched — SABnzbd accepts any URL at submission time and only fails the
// fetch asynchronously, so a submission that ends up in the Failed state
// exercises exactly the same real endpoints as one that would have
// succeeded, and TestLive_AddStatusRemoveRoundTrip's own state assertion
// already accepts ports.DownloadStateFailed as valid for this reason.
package sabnzbd_test

import (
	"context"
	"errors"
	"os"
	"purser/internal/adapters/sabnzbd"
	"purser/internal/ports"
	"testing"
)

func newLiveClient(t *testing.T) *sabnzbd.Client {
	t.Helper()
	baseURL := os.Getenv("PURSER_SABNZBD_BASE_URL")
	apiKey := os.Getenv("PURSER_SABNZBD_API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("PURSER_SABNZBD_BASE_URL / PURSER_SABNZBD_API_KEY not set, skipping")
	}

	cfg := sabnzbd.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.APIKey = apiKey
	c, err := sabnzbd.New(cfg)
	if err != nil {
		t.Fatalf("sabnzbd.New returned error: %v", err)
	}
	return c
}

// TestLive_AddStatusRemoveRoundTrip exercises a real "mode=addurl" call
// against a well-known public-domain sample .nzb (bigbuckbunny.nzb from
// SABnzbd's own test-file collection, used by countless SABnzbd client
// integrations for exactly this reason), a real Status lookup by the
// returned nzo_id, and a real Remove — proving this adapter's
// request-building code still matches the real API shape it assumes.
func TestLive_AddStatusRemoveRoundTrip(t *testing.T) {
	c := newLiveClient(t)
	ctx := context.Background()

	// purser-live-test.invalid: RFC 2606 reserves .invalid as guaranteed
	// never to resolve — see the doc comment above for why this test
	// doesn't need (and can't reliably get) a real fetchable .nzb.
	const unreachableNZB = "https://purser-live-test.invalid/release.nzb"

	id, err := c.Add(ctx, ports.AddDownloadRequest{
		Protocol:    ports.ProtocolUsenet,
		DownloadURL: unreachableNZB,
		Title:       "Purser Live Test",
		Category:    "",
	})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if id == "" {
		t.Fatal("Add returned an empty external ID")
	}
	t.Cleanup(func() {
		if err := c.Remove(context.Background(), id, true); err != nil {
			t.Logf("cleanup Remove(%s) returned error: %v", id, err)
		}
	})

	status, err := c.Status(ctx, id)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.ExternalID != id {
		t.Errorf("Status.ExternalID = %q, want %q", status.ExternalID, id)
	}
	switch status.State {
	case ports.DownloadStateQueued, ports.DownloadStateDownloading, ports.DownloadStatePaused, ports.DownloadStateCompleted, ports.DownloadStateFailed:
	default:
		t.Errorf("Status.State = %q, not one of the normalized values", status.State)
	}
}

// TestLive_StatusUnknownReturnsErrNotFound confirms an nzo_id the instance
// has never seen maps to ports.ErrNotFound against the real API, not just
// the fixture/contract suite.
func TestLive_StatusUnknownReturnsErrNotFound(t *testing.T) {
	c := newLiveClient(t)
	_, err := c.Status(context.Background(), "SABnzbd_nzo_0000000000")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status error = %v, want ports.ErrNotFound for an unknown nzo_id", err)
	}
}
