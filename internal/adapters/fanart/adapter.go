package fanart

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/pkg/httpclient"
	"strings"
)

// Compile-time interface check.
var _ ports.MetadataSource = (*Adapter)(nil)

const publicBaseURL = "https://webservice.fanart.tv/v3/"

// Adapter implements ports.MetadataSource for fanart.tv.
// Auth is via an api_key query parameter; no rate limit is enforced for personal keys.
type Adapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// New constructs a fanart.tv adapter from cfg.
func New(cfg config.MetadataSourceConfig) *Adapter {
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
		client:  httpclient.New(),
	}
}

// Name returns the identifier for this metadata source.
func (a *Adapter) Name() string { return string(domain.SourceFanart) }

// ContentTypes returns the content types this adapter can provide images for.
func (a *Adapter) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// ImagePriority returns 50 — fanart.tv is a secondary image source for music.
func (a *Adapter) ImagePriority() int { return 50 }

// get issues an authenticated GET to path (relative to baseURL) and decodes the JSON response into out.
func (a *Adapter) get(ctx context.Context, path string, out any) error {
	u, err := url.Parse(a.baseURL + path)
	if err != nil {
		return fmt.Errorf("fanart: parse url: %w", err)
	}
	q := u.Query()
	q.Set("api_key", a.apiKey)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("fanart: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return ports.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fanart: HTTP %d: %s", resp.StatusCode, string(b))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("fanart: decode: %w", err)
	}
	return nil
}
