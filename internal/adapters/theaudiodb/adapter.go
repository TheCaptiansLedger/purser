package theaudiodb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/pkg/cache"
	"purser/pkg/httpclient"
	"strings"
	"time"
)

// Compile-time interface assertions.
var (
	_ ports.MetadataSource     = (*Adapter)(nil)
	_ ports.StudioSearchSource = (*Adapter)(nil)
	_ ports.StudioThumbSource  = (*Adapter)(nil)
	_ ports.ExternalIDSource   = (*Adapter)(nil)
	_ ports.GroupContentSource = (*Adapter)(nil)
	_ ports.PersonImageSource  = (*Adapter)(nil)
)

const publicBaseURL = "https://www.theaudiodb.com/api/v1/json/"

// Adapter implements ports.MetadataSource for TheAudioDB.
// Auth is via the API key embedded in the URL path: /api/v1/json/{apikey}/.
// The free-tier key is "123"; Patreon subscribers receive a personal key.
type Adapter struct {
	baseURL string
	apiKey  string
	cache   *cache.Cache
	client  *http.Client
}

// New constructs a TheAudioDB adapter from cfg. c may be nil to disable caching.
func New(cfg config.MetadataSourceConfig, c *cache.Cache) *Adapter {
	base := cfg.URL
	if base == "" {
		base = publicBaseURL
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return &Adapter{
		baseURL: base,
		apiKey:  cfg.APIKey,
		cache:   c,
		client:  httpclient.New(),
	}
}

// Name returns the identifier for this metadata source.
func (a *Adapter) Name() string { return string(domain.SourceTheAudioDB) }

// ContentTypes returns the content types this adapter can provide data for.
func (a *Adapter) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// ImagePriority returns 100 — TheAudioDB is the preferred image source for music.
func (a *Adapter) ImagePriority() int { return 100 }

const cacheTTL = 24 * time.Hour

// get issues a GET to {baseURL}{apiKey}/{path} and decodes the JSON response into out.
// Responses are cached for 24 hours when a cache is configured.
func (a *Adapter) get(ctx context.Context, path string, out any) error {
	u := a.baseURL + a.apiKey + "/" + path

	if a.cache != nil {
		if v, ok := a.cache.Get(u); ok {
			return json.Unmarshal(v, out)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("theaudiodb: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return ports.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("theaudiodb: HTTP %d: %s", resp.StatusCode, string(b))
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("theaudiodb: read: %w", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("theaudiodb: decode: %w", err)
	}

	if a.cache != nil {
		a.cache.Set(u, b, cacheTTL)
	}
	return nil
}
