package lastfm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"sync"
	"time"
)

// Compile-time interface check.
var _ ports.MetadataSource = (*Adapter)(nil)

const (
	publicBaseURL  = "https://ws.audioscrobbler.com/2.0/"
	rateInterval   = 250 * time.Millisecond
	lfmPlaceholder = "2a96cbd8b46e442fc41c2b86b821562f"
)

// Adapter implements ports.MetadataSource for Last.fm.
// Auth is via an api_key query parameter on every request.
type Adapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
	limiter *rateLimiter
}

// New constructs a Last.fm adapter from cfg.
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
		client:  &http.Client{Timeout: 30 * time.Second},
		limiter: newRateLimiter(rateInterval),
	}
}

// Name returns the identifier for this metadata source.
func (a *Adapter) Name() string { return string(domain.SourceLastFM) }

// ContentTypes returns the content types this adapter can provide metadata for.
func (a *Adapter) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// ── Rate limiter ──────────────────────────────────────────────────────────────

// rateLimiter serializes outbound requests with a minimum gap between each.
type rateLimiter struct {
	mu      sync.Mutex
	lastReq time.Time
	minGap  time.Duration
}

func newRateLimiter(interval time.Duration) *rateLimiter {
	return &rateLimiter{minGap: interval}
}

func (rl *rateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	now := time.Now()
	target := rl.lastReq.Add(rl.minGap)
	if target.Before(now) {
		target = now
	}
	rl.lastReq = target
	rl.mu.Unlock()

	if d := time.Until(target); d > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
	return nil
}

// ── HTTP transport ────────────────────────────────────────────────────────────

func (a *Adapter) get(ctx context.Context, params url.Values, out any) error {
	if err := a.limiter.Wait(ctx); err != nil {
		return err
	}
	params.Set("api_key", a.apiKey)
	params.Set("format", "json")

	u, _ := url.Parse(a.baseURL)
	u.RawQuery = params.Encode()

	slog.Debug("lastfm: GET", "method", params.Get("method"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("lastfm: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return ports.ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("lastfm: HTTP %d: %s", resp.StatusCode, string(b))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("lastfm: decode: %w", err)
	}
	return nil
}

// bestImage returns the largest non-empty, non-placeholder URL from a Last.fm
// image array. Returns empty string when no usable image is found.
func bestImage(images []lfmImage) string {
	for i := len(images) - 1; i >= 0; i-- {
		if u := images[i].Text; u != "" && !strings.Contains(u, lfmPlaceholder) {
			return u
		}
	}
	return ""
}
