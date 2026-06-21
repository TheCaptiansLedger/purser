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
	"purser/pkg/httpclient"
	"strings"
)

var _ ports.MetadataSource = (*Adapter)(nil)

const publicBaseURL = "https://www.theaudiodb.com/api/v1/json/"

// Adapter implements ports.MetadataSource for TheAudioDB.
// Auth is via the API key embedded in the URL path: /api/v1/json/{apikey}/.
// The free-tier key is "123"; Patreon subscribers receive a personal key.
type Adapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// New constructs a TheAudioDB adapter from cfg.
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
func (a *Adapter) Name() string { return string(domain.SourceTheAudioDB) }

// ContentTypes returns the content types this adapter can provide data for.
func (a *Adapter) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// get issues a GET to {baseURL}{apiKey}/{path} and decodes the JSON response into out.
func (a *Adapter) get(ctx context.Context, path string, out any) error {
	u := a.baseURL + a.apiKey + "/" + path

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
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("theaudiodb: decode: %w", err)
	}
	return nil
}
