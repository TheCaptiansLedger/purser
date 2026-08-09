package fixtureserver_test

import (
	"context"
	"errors"
	"purser/internal/adapters/qbittorrent"
	"purser/internal/adapters/qbittorrent/fixtureserver"
	"purser/internal/ports"
	"testing"
)

func newFixtureClient(t *testing.T) *qbittorrent.Client {
	t.Helper()
	cfg := qbittorrent.DefaultConfig()
	cfg.BaseURL = "http://qbittorrent.invalid"
	cfg.Username = "fixture-user"
	cfg.Password = "fixture-pass"
	c, err := qbittorrent.New(cfg, qbittorrent.WithBaseTransport(fixtureserver.Transport()), qbittorrent.WithTagLookupInterval(0))
	if err != nil {
		t.Fatalf("qbittorrent.New returned error: %v", err)
	}
	return c
}

func TestTransport_Add_ReturnsKnownHash(t *testing.T) {
	c := newFixtureClient(t)

	id, err := c.Add(context.Background(), ports.AddDownloadRequest{DownloadURL: "magnet:?xt=urn:btih:abc", Title: "K6 Fixture", Category: "movies"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if id != fixtureserver.KnownHash {
		t.Errorf("Add id = %q, want %q", id, fixtureserver.KnownHash)
	}
}

func TestTransport_Status_KnownHashAfterAdd(t *testing.T) {
	c := newFixtureClient(t)

	if _, err := c.Add(context.Background(), ports.AddDownloadRequest{DownloadURL: "magnet:?xt=urn:btih:abc"}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	status, err := c.Status(context.Background(), fixtureserver.KnownHash)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.ExternalID != fixtureserver.KnownHash || status.State != ports.DownloadStateDownloading {
		t.Errorf("Status = %+v, want ExternalID=%s State=downloading", status, fixtureserver.KnownHash)
	}
}

func TestTransport_Status_UnknownHashReturnsNotFound(t *testing.T) {
	c := newFixtureClient(t)

	_, err := c.Status(context.Background(), "no-such-hash")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status error = %v, want wrapping ports.ErrNotFound", err)
	}
}

func TestTransport_Remove_ThenStatusReturnsNotFound(t *testing.T) {
	c := newFixtureClient(t)

	if _, err := c.Add(context.Background(), ports.AddDownloadRequest{DownloadURL: "magnet:?xt=urn:btih:abc"}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if err := c.Remove(context.Background(), fixtureserver.KnownHash, false); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	_, err := c.Status(context.Background(), fixtureserver.KnownHash)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status after Remove error = %v, want wrapping ports.ErrNotFound", err)
	}
}
