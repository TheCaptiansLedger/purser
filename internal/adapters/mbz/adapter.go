package mbz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Compile-time interface check.
var _ ports.MetadataSource = (*Adapter)(nil)

var errNotFound = errors.New("musicbrainz: not found")

const (
	publicBaseURL = "https://musicbrainz.org/ws/2/"
	defaultUA     = "purser/1.0 (https://github.com/thecaptiansledger/purser)"
)

// Adapter implements ports.MetadataSource for MusicBrainz.
// MusicBrainz is a public API requiring no auth but enforcing 1 req/sec.
type Adapter struct {
	baseURL   string
	userAgent string
	client    *http.Client
	limiter   *rateLimiter
}

// New constructs a MusicBrainz adapter from cfg.
// If cfg.URL is empty the public MusicBrainz endpoint is used.
func New(cfg config.MetadataSourceConfig) *Adapter {
	base := cfg.URL
	if base == "" {
		base = publicBaseURL
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	ua := cfg.UserAgent
	if ua == "" {
		ua = defaultUA
	}
	return &Adapter{
		baseURL:   base,
		userAgent: ua,
		client:    &http.Client{Timeout: 30 * time.Second},
		limiter:   newRateLimiter(time.Second),
	}
}

// Name returns the identifier for this metadata source.
func (a *Adapter) Name() string { return string(domain.SourceMusicBrainz) }

// ContentTypes returns the content types this adapter can provide metadata for.
func (a *Adapter) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// ── Rate limiter ──────────────────────────────────────────────────────────────

// rateLimiter serializes outbound requests with a minimum gap between each.
// Concurrent callers each claim a future time slot under the lock, then sleep
// until their slot — no two requests fire within minGap of each other.
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

func (a *Adapter) get(ctx context.Context, url string, out any) error {
	if err := a.limiter.Wait(ctx); err != nil {
		return err
	}
	return a.doGet(ctx, url, out)
}

func (a *Adapter) doGet(ctx context.Context, rawURL string, out any) error {
	slog.Debug("mbz: GET", "url", rawURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", a.userAgent)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("musicbrainz: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		wait := retryAfterDuration(resp.Header.Get("Retry-After"))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		return a.doGet(ctx, rawURL, out)
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("musicbrainz: HTTP %d: %s", resp.StatusCode, string(b))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("musicbrainz: decode: %w", err)
	}
	return nil
}

func retryAfterDuration(header string) time.Duration {
	if secs, err := strconv.Atoi(header); err == nil {
		return time.Duration(secs) * time.Second
	}
	return time.Second
}
