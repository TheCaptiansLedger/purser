package mbz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/pkg/cache"
	"purser/pkg/httpclient"
	"strconv"
	"strings"
	"sync"
	"time"
)

const cacheTTL = 24 * time.Hour

// Compile-time interface assertions.
var (
	_ ports.MetadataSource     = (*Adapter)(nil)
	_ ports.StudioSearchSource = (*Adapter)(nil)
	_ ports.PeopleSearchSource = (*Adapter)(nil)
	_ ports.PersonRoleSource   = (*Adapter)(nil)
	_ ports.ItemSearchSource   = (*Adapter)(nil)
	_ ports.ExternalIDSource   = (*Adapter)(nil)
	_ ports.EntryContentSource = (*Adapter)(nil)
	_ ports.GroupContentSource = (*Adapter)(nil)
	_ ports.EntryPeopleSource  = (*Adapter)(nil)
)

// ImagePriority returns 0 — MusicBrainz does not provide images.
func (a *Adapter) ImagePriority() int { return 0 }

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
	cache     *cache.Cache
	client    *http.Client
	limiter   *rateLimiter
}

// New constructs a MusicBrainz adapter from cfg. c may be nil to disable caching.
func New(cfg config.MetadataSourceConfig, c *cache.Cache) *Adapter {
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
		cache:     c,
		client:    httpclient.New(),
		limiter:   newRateLimiter(time.Second),
	}
}

// Name returns the identifier for this metadata source.
func (a *Adapter) Name() string { return string(domain.SourceMusicBrainz) }

// ContentTypes returns the content types this adapter can provide metadata for.
func (a *Adapter) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

// PersonRoles declares the person roles MusicBrainz covers (individual artists).
func (a *Adapter) PersonRoles() []domain.PersonRole {
	return []domain.PersonRole{domain.RoleArtist, domain.RoleProducer}
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
	if a.cache != nil {
		if v, ok := a.cache.Get(url); ok {
			return json.Unmarshal(v, out)
		}
	}

	if err := a.limiter.Wait(ctx); err != nil {
		return err
	}

	b, err := a.getRaw(ctx, url, out)
	if err != nil {
		return err
	}

	if a.cache != nil {
		a.cache.Set(url, b, cacheTTL)
	}
	return nil
}

// getRaw fetches url, decodes into out, and returns the raw response bytes for caching.
//
//nolint:cyclop // MBZ rate-limit retry handling requires branching on multiple HTTP status codes.
func (a *Adapter) getRaw(ctx context.Context, rawURL string, out any) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", a.userAgent)

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}

	if resp.StatusCode == http.StatusBadRequest {
		b, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(b), "Invalid mbid") {
			return nil, errNotFound
		}
		return nil, fmt.Errorf("musicbrainz: HTTP %d: %s", resp.StatusCode, string(b))
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		wait := retryAfterDuration(resp.Header.Get("Retry-After"))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		return a.getRaw(ctx, rawURL, out)
	}

	if resp.StatusCode == http.StatusServiceUnavailable {
		b, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(b), "rate limit") {
			wait := 5 * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(wait):
			}
			return a.getRaw(ctx, rawURL, out)
		}
		return nil, fmt.Errorf("musicbrainz: HTTP %d: %s", resp.StatusCode, string(b))
	}

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("musicbrainz: HTTP %d: %s", resp.StatusCode, string(b))
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: read: %w", err)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return nil, fmt.Errorf("musicbrainz: decode: %w", err)
	}
	return b, nil
}

func retryAfterDuration(header string) time.Duration {
	if secs, err := strconv.Atoi(header); err == nil {
		return time.Duration(secs) * time.Second
	}
	return time.Second
}
