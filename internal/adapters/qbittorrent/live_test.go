//go:build live

// This file is gated behind the "live" build tag — it never runs under
// plain `go test ./...` or in CI, only via `go test -tags=live`. It hits a
// real, operator-run qBittorrent instance to catch schema drift, per
// docs/adr/0003-go-testing-standards.md's "a real, live-network
// verification test may exist for that adapter and that adapter only,
// gated behind a build tag" rule. Requires PURSER_QBITTORRENT_BASE_URL,
// PURSER_QBITTORRENT_USERNAME, and PURSER_QBITTORRENT_PASSWORD env vars
// (matching internal/config.QBittorrent's actual "qbittorrent.base_url"/
// "qbittorrent.username"/"qbittorrent.password" Viper keys) pointed at a
// real qBittorrent instance; skips if any is missing. Run manually — there
// is no public default instance this could run against unattended.
//
// TestLive_AddStatusRemoveRoundTrip submits a real, tiny, well-known
// magnet link (a public-domain test torrent) so it doesn't depend on any
// particular indexer's catalog, then removes it again (with deleteFiles)
// so the operator's instance isn't left with test leftovers.
package qbittorrent_test

import (
	"context"
	"errors"
	"os"
	"purser/internal/adapters/qbittorrent"
	"purser/internal/ports"
	"testing"
)

func newLiveClient(t *testing.T) *qbittorrent.Client {
	t.Helper()
	baseURL := os.Getenv("PURSER_QBITTORRENT_BASE_URL")
	username := os.Getenv("PURSER_QBITTORRENT_USERNAME")
	password := os.Getenv("PURSER_QBITTORRENT_PASSWORD")
	if baseURL == "" || username == "" || password == "" {
		t.Skip("PURSER_QBITTORRENT_BASE_URL / PURSER_QBITTORRENT_USERNAME / PURSER_QBITTORRENT_PASSWORD not set, skipping")
	}

	cfg := qbittorrent.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Username = username
	cfg.Password = password
	c, err := qbittorrent.New(cfg)
	if err != nil {
		t.Fatalf("qbittorrent.New returned error: %v", err)
	}
	return c
}

// TestLive_AddStatusRemoveRoundTrip exercises the real login/session-cookie
// flow, a real Add (the well-known "Big Buck Bunny" public-domain
// promotional magnet, used by countless BitTorrent client test suites for
// exactly this reason), a real Status lookup by the returned hash, and a
// real Remove — proving this adapter's request-building code still matches
// the real WebUI API shape it assumes.
func TestLive_AddStatusRemoveRoundTrip(t *testing.T) {
	c := newLiveClient(t)
	ctx := context.Background()

	const bigBuckBunnyMagnet = "magnet:?xt=urn:btih:dd8255ecdc7ca55fb0bbf81323d87062db1f6d1c&dn=Big+Buck+Bunny"

	id, err := c.Add(ctx, ports.AddDownloadRequest{
		Protocol:    ports.ProtocolTorrent,
		DownloadURL: bigBuckBunnyMagnet,
		Title:       "Big Buck Bunny",
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

// TestLive_StatusUnknownReturnsErrNotFound confirms a hash the instance has
// never seen maps to ports.ErrNotFound against the real API, not just the
// fixture/contract suite.
func TestLive_StatusUnknownReturnsErrNotFound(t *testing.T) {
	c := newLiveClient(t)
	_, err := c.Status(context.Background(), "0000000000000000000000000000000000000000")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status error = %v, want ports.ErrNotFound for an unknown hash", err)
	}
}
