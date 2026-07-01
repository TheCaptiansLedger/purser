package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"purser/internal/ports"
	"purser/pkg/cache"
	"purser/pkg/httpclient"
	"time"
)

const (
	defaultBaseURL  = "https://api.github.com"
	ttlIssues       = 5 * time.Minute
	ttlReleases     = 5 * time.Minute
	ttlContributors = time.Hour
)

var _ ports.GitHubProxy = (*Adapter)(nil)

// Config holds connection settings for the GitHub adapter.
type Config struct {
	Repo  string // e.g. "owner/repo"
	Token string // optional; lifts rate limit from 60 to 5000 req/hr when set
}

// Adapter implements ports.GitHubProxy with server-side LRU caching.
type Adapter struct {
	baseURL string
	repo    string
	token   string
	cache   *cache.Cache
	client  *http.Client
}

// New returns an Adapter for the given GitHub config.
func New(cfg Config, c *cache.Cache) *Adapter {
	return newAdapter(cfg, c, defaultBaseURL)
}

// NewWithBaseURL creates an Adapter with a custom base URL. Used in tests to
// point the adapter at an httptest.Server instead of api.github.com.
func NewWithBaseURL(cfg Config, c *cache.Cache, baseURL string) *Adapter {
	return newAdapter(cfg, c, baseURL)
}

func newAdapter(cfg Config, c *cache.Cache, baseURL string) *Adapter {
	return &Adapter{
		baseURL: baseURL,
		repo:    cfg.Repo,
		token:   cfg.Token,
		cache:   c,
		client:  httpclient.New(),
	}
}

// Issues proxies GET /repos/{repo}/issues with state and optional label filters.
func (a *Adapter) Issues(ctx context.Context, state, label string) (json.RawMessage, error) {
	key := "issues:" + state
	if label != "" {
		key += ":" + label
	}
	if v, ok := a.cache.Get(key); ok {
		return json.RawMessage(v), nil
	}

	q := url.Values{"state": {state}, "per_page": {"100"}}
	if label != "" {
		q.Set("labels", label)
	}
	raw, err := a.get(ctx, fmt.Sprintf("/repos/%s/issues?%s", a.repo, q.Encode()))
	if err != nil {
		return nil, err
	}
	a.cache.Set(key, raw, ttlIssues)
	return json.RawMessage(raw), nil
}

// Releases proxies GET /repos/{repo}/releases.
func (a *Adapter) Releases(ctx context.Context) (json.RawMessage, error) {
	const key = "releases"
	if v, ok := a.cache.Get(key); ok {
		return json.RawMessage(v), nil
	}
	raw, err := a.get(ctx, fmt.Sprintf("/repos/%s/releases", a.repo))
	if err != nil {
		return nil, err
	}
	a.cache.Set(key, raw, ttlReleases)
	return json.RawMessage(raw), nil
}

// Contributors returns a flat JSON array of commit objects for the range base..head.
// If base is empty, commits reachable from head are returned.
// The response is normalised so the caller always receives an array regardless of
// which upstream GitHub endpoint was used.
func (a *Adapter) Contributors(ctx context.Context, base, head string) (json.RawMessage, error) {
	key := "contributors:" + base + ":" + head
	if v, ok := a.cache.Get(key); ok {
		return json.RawMessage(v), nil
	}

	var raw []byte
	var err error
	if base != "" {
		raw, err = a.compareCommits(ctx, base, head)
	} else {
		raw, err = a.get(ctx, fmt.Sprintf("/repos/%s/commits?sha=%s&per_page=100",
			a.repo, url.QueryEscape(head)))
	}
	if err != nil {
		return nil, err
	}
	a.cache.Set(key, raw, ttlContributors)
	return json.RawMessage(raw), nil
}

// compareCommits calls /compare/{base}...{head} and extracts the commits array.
func (a *Adapter) compareCommits(ctx context.Context, base, head string) ([]byte, error) {
	path := fmt.Sprintf("/repos/%s/compare/%s...%s",
		a.repo, url.PathEscape(base), url.PathEscape(head))
	raw, err := a.get(ctx, path)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Commits json.RawMessage `json:"commits"`
	}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, fmt.Errorf("github: decode compare response: %w", err)
	}
	if wrapper.Commits == nil {
		return []byte("[]"), nil
	}
	return wrapper.Commits, nil
}

func (a *Adapter) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("github: rate limited — try again in a minute")
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github: HTTP %d: %s", resp.StatusCode, string(b))
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("github: read response: %w", err)
	}
	return b, nil
}
