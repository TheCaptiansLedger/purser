package stashdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
)

// Compile-time interface check.
var _ ports.MetadataSource = (*Adapter)(nil)

const (
	publicURL = "https://stashdb.org/graphql"
	defaultUA = "purser/1.0 (https://github.com/purser-app/purser)"
)

// Adapter implements ports.MetadataSource for StashDB.
// StashDB is a community-run stash-box instance covering adult and JAV content.
type Adapter struct {
	apiKey    string
	baseURL   string
	userAgent string
	client    *http.Client
}

// New constructs a StashDB adapter from cfg.
// If cfg.URL is empty the public StashDB endpoint is used.
func New(cfg config.MetadataSourceConfig) *Adapter {
	base := cfg.URL
	if base == "" {
		base = publicURL
	}
	ua := cfg.UserAgent
	if ua == "" {
		ua = defaultUA
	}
	return &Adapter{
		apiKey:    cfg.APIKey,
		baseURL:   base,
		userAgent: ua,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *Adapter) Name() string                       { return "stashdb" }
func (a *Adapter) ContentTypes() []domain.ContentType { return []domain.ContentType{domain.ContentTypeAdult, domain.ContentTypeJAV} }

// ── GraphQL transport ─────────────────────────────────────────────────────────

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

func (a *Adapter) gql(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(gqlRequest{Query: query, Variables: vars})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", a.userAgent)
	if a.apiKey != "" {
		req.Header.Set("ApiKey", a.apiKey)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("stashdb: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stashdb: HTTP %d: %s", resp.StatusCode, string(b))
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("stashdb: decode: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("stashdb: %s", envelope.Errors[0].Message)
	}
	return json.Unmarshal(envelope.Data, out)
}
