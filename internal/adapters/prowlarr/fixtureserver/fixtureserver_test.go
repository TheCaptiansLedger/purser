package fixtureserver_test

import (
	"context"
	"purser/internal/adapters/prowlarr"
	"purser/internal/adapters/prowlarr/fixtureserver"
	"purser/internal/ports"
	"testing"
)

func newFixtureClient(t *testing.T) *prowlarr.Client {
	t.Helper()
	cfg := prowlarr.DefaultConfig()
	cfg.BaseURL = "http://prowlarr.invalid/api/v1"
	cfg.APIKey = "fixture-key"
	c, err := prowlarr.New(cfg, prowlarr.WithBaseTransport(fixtureserver.Transport()))
	if err != nil {
		t.Fatalf("prowlarr.New returned error: %v", err)
	}
	return c
}

func TestTransport_Search_KnownQueryReturnsOneRelease(t *testing.T) {
	c := newFixtureClient(t)

	results, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: fixtureserver.KnownQuery})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search = %+v, want one result", results)
	}
	r := results[0]
	if r.GUID != fixtureserver.KnownGUID || r.Title != fixtureserver.KnownTitle {
		t.Errorf("result = %+v, want guid=%s title=%s", r, fixtureserver.KnownGUID, fixtureserver.KnownTitle)
	}
	if r.Protocol != ports.ProtocolTorrent {
		t.Errorf("Protocol = %q, want %q", r.Protocol, ports.ProtocolTorrent)
	}
	if r.PublishDate.IsZero() {
		t.Error("PublishDate is zero, want the fixture's parsed date")
	}
}

func TestTransport_Search_UnknownQueryReturnsEmptyNotError(t *testing.T) {
	c := newFixtureClient(t)

	results, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: "no-such-release"})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search = %+v, want empty", results)
	}
}
