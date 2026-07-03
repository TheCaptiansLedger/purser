package identifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

const acoustidBaseURL = "https://api.acoustid.org"

type acoustidClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

func newAcoustIDClient(apiKey string) *acoustidClient {
	return &acoustidClient{
		apiKey:  apiKey,
		baseURL: acoustidBaseURL,
		http:    &http.Client{},
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

// acoustidResponse is the top-level response from the AcoustID lookup API.
type acoustidResponse struct {
	Status  string           `json:"status"`
	Results []acoustidResult `json:"results"`
}

type acoustidResult struct {
	Recordings []acoustidRecording `json:"recordings"`
	Score      float64             `json:"score"`
}

type acoustidRecording struct {
	ID string `json:"id"` // MusicBrainz Recording ID
}

// Lookup queries the AcoustID API and returns a list of MusicBrainz Recording IDs
// that match the given raw fingerprint and duration (in seconds).
// Returns an empty slice (not an error) when no matches are found.
func (c *acoustidClient) Lookup(ctx context.Context, fingerprint string, durationSecs int) ([]string, error) {
	params := url.Values{}
	params.Set("client", c.apiKey)
	params.Set("meta", "recordings")
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

	var mbids []string
	for _, r := range result.Results {
		for _, rec := range r.Recordings {
			if rec.ID != "" {
				mbids = append(mbids, rec.ID)
			}
		}
	}
	return mbids, nil
}
