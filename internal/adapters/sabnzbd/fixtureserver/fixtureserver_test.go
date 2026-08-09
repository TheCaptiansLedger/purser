package fixtureserver_test

import (
	"context"
	"errors"
	"purser/internal/adapters/sabnzbd"
	"purser/internal/adapters/sabnzbd/fixtureserver"
	"purser/internal/ports"
	"testing"
)

func newFixtureClient(t *testing.T) *sabnzbd.Client {
	t.Helper()
	cfg := sabnzbd.DefaultConfig()
	cfg.BaseURL = "http://sabnzbd.invalid"
	cfg.APIKey = "fixture-key"
	c, err := sabnzbd.New(cfg, sabnzbd.WithBaseTransport(fixtureserver.Transport()))
	if err != nil {
		t.Fatalf("sabnzbd.New returned error: %v", err)
	}
	return c
}

func TestTransport_Add_ReturnsKnownNzoID(t *testing.T) {
	c := newFixtureClient(t)

	id, err := c.Add(context.Background(), ports.AddDownloadRequest{DownloadURL: "http://example.invalid/fixture.nzb", Title: "K6 Fixture", Category: "movies"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if id != fixtureserver.KnownNzoID {
		t.Errorf("Add id = %q, want %q", id, fixtureserver.KnownNzoID)
	}
}

func TestTransport_Status_KnownNzoIDAfterAdd(t *testing.T) {
	c := newFixtureClient(t)

	if _, err := c.Add(context.Background(), ports.AddDownloadRequest{DownloadURL: "http://example.invalid/fixture.nzb"}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	status, err := c.Status(context.Background(), fixtureserver.KnownNzoID)
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if status.ExternalID != fixtureserver.KnownNzoID || status.State != ports.DownloadStateDownloading {
		t.Errorf("Status = %+v, want ExternalID=%s State=downloading", status, fixtureserver.KnownNzoID)
	}
}

func TestTransport_Status_UnknownNzoIDReturnsNotFound(t *testing.T) {
	c := newFixtureClient(t)

	_, err := c.Status(context.Background(), "no-such-id")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status error = %v, want wrapping ports.ErrNotFound", err)
	}
}

func TestTransport_Remove_ThenStatusReturnsNotFound(t *testing.T) {
	c := newFixtureClient(t)

	if _, err := c.Add(context.Background(), ports.AddDownloadRequest{DownloadURL: "http://example.invalid/fixture.nzb"}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if err := c.Remove(context.Background(), fixtureserver.KnownNzoID, false); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	_, err := c.Status(context.Background(), fixtureserver.KnownNzoID)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Status after Remove error = %v, want wrapping ports.ErrNotFound", err)
	}
}
