package identifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"purser/pkg/cache"
	"strconv"
	"time"
)

const (
	acoustidBaseURL  = "https://api.acoustid.org"
	acoustidCacheTTL = 7 * 24 * time.Hour
)

type acoustidClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
	cache   *cache.Cache // nil = caching disabled (tests)
}

func newAcoustIDClient(apiKey string, c *cache.Cache) *acoustidClient {
	return &acoustidClient{
		apiKey:  apiKey,
		baseURL: acoustidBaseURL,
		http:    &http.Client{},
		cache:   c,
	}
}

// NewTestAcoustIDClient constructs a client that points at baseURL instead of
// the public AcoustID endpoint. Used by unit tests with httptest.Server.
func NewTestAcoustIDClient(apiKey, baseURL string) *acoustidClient {
	return &acoustidClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

// NewTestAcoustIDClientWithCache constructs a test client with caching enabled.
// Used by tests that verify cache hit/miss behaviour.
func NewTestAcoustIDClientWithCache(apiKey, baseURL string, c *cache.Cache) *acoustidClient {
	return &acoustidClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		http:    &http.Client{},
		cache:   c,
	}
}

// ── AcoustID API response types ───────────────────────────────────────────────

type acoustidResponse struct {
	Status  string           `json:"status"`
	Results []acoustidResult `json:"results"`
}

type acoustidResult struct {
	ID         string              `json:"id"` // AcoustID identifier UUID
	Score      float64             `json:"score"`
	Recordings []acoustidRecording `json:"recordings"`
}

type acoustidRecording struct {
	ID       string            `json:"id"` // MusicBrainz Recording ID
	Title    string            `json:"title"`
	Duration float64           `json:"duration"` // fractional seconds (AcoustID returns floats)
	Artists  []acoustidArtist  `json:"artists"`
	Releases []acoustidRelease `json:"releases"`
}

type acoustidArtist struct {
	Name string `json:"name"`
}

type acoustidRelease struct {
	Title string `json:"title"`
}

// ── Exported result types ─────────────────────────────────────────────────────

// AcoustIDRecording holds inline metadata for a single MusicBrainz recording
// returned by AcoustID. Enough to rank candidates against embedded tags without
// additional MusicBrainz lookups.
type AcoustIDRecording struct {
	MBID     string // MusicBrainz Recording ID
	Title    string
	Duration int      // seconds
	Artist   string   // first artist credit name
	Albums   []string // associated release titles
}

// AcoustIDMatch is one result from the AcoustID /v2/lookup endpoint.
// A single fingerprint may produce multiple results at different scores.
type AcoustIDMatch struct {
	AcoustID   string              // AcoustID identifier UUID (may be empty)
	Score      float64             // fingerprint match confidence, 0.0–1.0
	Recordings []AcoustIDRecording // candidate recordings with inline metadata
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func toAcoustIDRecording(rec acoustidRecording) (AcoustIDRecording, bool) {
	if rec.ID == "" {
		return AcoustIDRecording{}, false
	}
	ar := AcoustIDRecording{MBID: rec.ID, Title: rec.Title, Duration: int(rec.Duration)}
	if len(rec.Artists) > 0 {
		ar.Artist = rec.Artists[0].Name
	}
	for _, rel := range rec.Releases {
		if rel.Title != "" {
			ar.Albums = append(ar.Albums, rel.Title)
		}
	}
	return ar, true
}

// ── Client ────────────────────────────────────────────────────────────────────

// Lookup queries the AcoustID API and returns matches with their scores and
// inline recording metadata. Results are ordered by descending score.
// Returns an empty slice (not an error) when no matches are found.
func (c *acoustidClient) Lookup(ctx context.Context, fingerprint string, durationSecs int) ([]AcoustIDMatch, error) {
	cacheKey := fingerprint + ":" + strconv.Itoa(durationSecs)
	if c.cache != nil {
		if v, ok := c.cache.Get(cacheKey); ok {
			var matches []AcoustIDMatch
			_ = json.Unmarshal(v, &matches)
			return matches, nil
		}
	}

	params := url.Values{}
	params.Set("client", c.apiKey)
	params.Set("meta", "recordings releases")
	params.Set("fingerprint", fingerprint)
	params.Set("duration", strconv.Itoa(durationSecs))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v2/lookup?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("acoustid lookup: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("acoustid status %d", resp.StatusCode)
	}

	var result acoustidResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode acoustid response: %w", err)
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("acoustid status: %s", result.Status)
	}

	matches := make([]AcoustIDMatch, 0, len(result.Results))
	for _, r := range result.Results {
		m := AcoustIDMatch{AcoustID: r.ID, Score: r.Score}
		for _, rec := range r.Recordings {
			if ar, ok := toAcoustIDRecording(rec); ok {
				m.Recordings = append(m.Recordings, ar)
			}
		}
		if len(m.Recordings) > 0 {
			matches = append(matches, m)
		}
	}

	if c.cache != nil {
		if b, err := json.Marshal(matches); err == nil {
			c.cache.Set(cacheKey, b, acoustidCacheTTL)
		}
	}

	return matches, nil
}
