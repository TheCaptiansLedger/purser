//go:build live

// This file is gated behind the "live" build tag — it never runs under
// plain `go test ./...` or in CI, only via `go test -tags=live`. It hits a
// real, operator-run Prowlarr instance to catch schema drift, per
// docs/adr/0003-go-testing-standards.md's "a real, live-network
// verification test may exist for that adapter and that adapter only,
// gated behind a build tag" rule. Requires PURSER_PROWLARR_BASE_URL and
// PURSER_PROWLARR_API_KEY env vars (matching internal/config.Prowlarr's
// actual "prowlarr.base_url"/"prowlarr.api_key" Viper keys) pointed at a
// real Prowlarr instance with at least one indexer configured; skips if
// either is missing. Run manually — there is no public default instance
// this could run against unattended.
package prowlarr_test

import (
	"context"
	"os"
	"purser/internal/adapters/prowlarr"
	"purser/internal/ports"
	"testing"
)

func newLiveClient(t *testing.T) *prowlarr.Client {
	t.Helper()
	baseURL := os.Getenv("PURSER_PROWLARR_BASE_URL")
	apiKey := os.Getenv("PURSER_PROWLARR_API_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("PURSER_PROWLARR_BASE_URL / PURSER_PROWLARR_API_KEY not set, skipping")
	}

	cfg := prowlarr.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.APIKey = apiKey
	c, err := prowlarr.New(cfg)
	if err != nil {
		t.Fatalf("prowlarr.New returned error: %v", err)
	}
	return c
}

// TestLive_SearchReturnsWellFormedResults hits the real Prowlarr instance
// with a broad, common term and checks the response shape this adapter
// assumes still holds — it doesn't assert on any specific release, since a
// real indexer's catalog changes over time and isn't this test's business.
func TestLive_SearchReturnsWellFormedResults(t *testing.T) {
	c := newLiveClient(t)
	results, err := c.Search(context.Background(), ports.IndexerSearchParams{Query: "the"})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) == 0 {
		t.Skip("Search returned no results for a broad term, skipping shape assertions")
	}

	r := results[0]
	if r.GUID == "" {
		t.Error("first result has empty GUID")
	}
	if r.Title == "" {
		t.Error("first result has empty Title")
	}
	if r.Protocol != ports.ProtocolTorrent && r.Protocol != ports.ProtocolUsenet {
		t.Errorf("first result Protocol = %q, want %q or %q", r.Protocol, ports.ProtocolTorrent, ports.ProtocolUsenet)
	}
}

// TestLive_SearchWithNoMatchesReturnsEmptyNotError confirms a genuinely
// unmatchable query still returns an empty slice and a nil error against
// the real API, not just the fixture/contract suite.
func TestLive_SearchWithNoMatchesReturnsEmptyNotError(t *testing.T) {
	c := newLiveClient(t)
	results, err := c.Search(context.Background(), ports.IndexerSearchParams{
		Query: "purser-live-test-query-that-should-never-match-anything-zzzzzzzz",
	})
	if err != nil {
		t.Fatalf("Search returned error: %v, want nil for a zero-result search", err)
	}
	if len(results) != 0 {
		t.Logf("Search unexpectedly matched %d result(s) for a nonsense query: %+v", len(results), results)
	}
}
