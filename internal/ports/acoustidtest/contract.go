// Package acoustidtest is the shared contract test suite for the
// ports.AcoustIDClient port's Lookup method. Fingerprint is local-only
// (shells out to fpcalc, no network) and is not part of this suite — it's
// tested directly in the adapter's own package against the real binary,
// per docs/adr/0003-go-testing-standards.md. Like musicbrainztest, this
// suite runs its own httptest.Server serving small canned fixtures and
// hands the adapter constructor that server's URL; richer field-mapping
// assertions against real recorded AcoustID responses belong in the
// adapter's own package.
package acoustidtest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/ports"
	"testing"
)

// Known/unknown fingerprints the fixture server recognizes — any other
// value yields an empty result set, exactly like the real AcoustID API
// reports an unmatched fingerprint.
const (
	KnownFingerprint   = "AQADtEmybGmnJEmSJEmSHEk"
	UnknownFingerprint = "AQADdEeeeeee0000000000E"
	KnownDurationSecs  = 200.0
)

// NewClientFunc constructs a fresh ports.AcoustIDClient pointed at baseURL
// for the duration of a single subtest.
type NewClientFunc func(t *testing.T, baseURL string) ports.AcoustIDClient

// TestAcoustIDClient runs the shared AcoustIDClient.Lookup contract against
// newClient, using a fixture HTTP server this suite owns.
func TestAcoustIDClient(t *testing.T, newClient NewClientFunc) {
	t.Helper()

	server := httptest.NewServer(fixtureHandler())
	t.Cleanup(server.Close)

	t.Run("Lookup returns matches for a known fingerprint", func(t *testing.T) { testLookupKnown(t, newClient, server.URL) })
	t.Run("Lookup returns ErrNotFound for an unknown fingerprint", func(t *testing.T) { testLookupUnknown(t, newClient, server.URL) })
}

func testLookupKnown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	matches, err := c.Lookup(context.Background(), KnownFingerprint, KnownDurationSecs)
	if err != nil {
		t.Fatalf("Lookup returned error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("Lookup returned %d matches, want 1", len(matches))
	}
	m := matches[0]
	if m.AcoustID == "" {
		t.Error("match AcoustID is empty")
	}
	if len(m.Recordings) != 1 || m.Recordings[0].Title != "Please Please Me" {
		t.Fatalf("Recordings = %+v, want one recording titled Please Please Me", m.Recordings)
	}
	if len(m.Recordings[0].ReleaseGroups) != 1 || m.Recordings[0].ReleaseGroups[0].Title != "Please Please Me" {
		t.Fatalf("ReleaseGroups = %+v, want one release group titled Please Please Me", m.Recordings[0].ReleaseGroups)
	}
}

func testLookupUnknown(t *testing.T, newClient NewClientFunc, baseURL string) {
	c := newClient(t, baseURL)
	_, err := c.Lookup(context.Background(), UnknownFingerprint, KnownDurationSecs)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Lookup error = %v, want wrapping ErrNotFound", err)
	}
}

func fixtureHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fp := r.URL.Query().Get("fingerprint")
		w.Header().Set("Content-Type", "application/json")

		if fp != KnownFingerprint {
			_ = json.NewEncoder(w).Encode(struct {
				Status  string                `json:"status"`
				Results []ports.AcoustIDMatch `json:"results"`
			}{Status: "ok", Results: []ports.AcoustIDMatch{}})
			return
		}

		_ = json.NewEncoder(w).Encode(struct {
			Status  string                `json:"status"`
			Results []ports.AcoustIDMatch `json:"results"`
		}{
			Status: "ok",
			Results: []ports.AcoustIDMatch{
				{
					AcoustID: "9ff43b6a-4f16-4b64-a4a2-15d2374f1e02",
					Score:    0.9,
					Recordings: []ports.AcoustIDRecording{
						{
							MBID:  "b720b11f-2c00-4f48-83b9-4aa9d3207e8c",
							Title: "Please Please Me",
							ReleaseGroups: []ports.AcoustIDReleaseGroup{
								{
									MBID:  "de208292-8db5-3aed-a14a-b37a84d8c521",
									Title: "Please Please Me",
									Type:  "Album",
								},
							},
						},
					},
				},
			},
		})
	})

	return mux
}
