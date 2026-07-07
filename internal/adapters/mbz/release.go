package mbz

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"purser/internal/domain"
	"purser/internal/ports"
)

// ── MusicBrainz response types ────────────────────────────────────────────────

type mbzReleaseList struct {
	Releases []mbzReleaseRef `json:"releases"`
}

type mbzReleaseRef struct {
	ID string `json:"id"`
}

type mbzReleaseDetail struct {
	Media []mbzMedium `json:"media"`
}

type mbzMedium struct {
	TrackCount int        `json:"track-count"`
	Tracks     []mbzTrack `json:"tracks"`
}

type mbzTrack struct {
	Number    string `json:"number"` // track position as string, e.g. "1" or "A1" for vinyl
	Title     string `json:"title"`
	Recording struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Length int    `json:"length"` // milliseconds
	} `json:"recording"`
}

// mbzFullRelease is a release in the release-group browse or barcode search response.
type mbzFullRelease struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Status    string         `json:"status"`
	Country   string         `json:"country"`
	Date      string         `json:"date"`
	Barcode   string         `json:"barcode"`
	LabelInfo []mbzLabelInfo `json:"label-info"`
	Media     []mbzRGMedium  `json:"media"`
}

// mbzRGMedium carries the format and track count for a single disc.
type mbzRGMedium struct {
	Format     string `json:"format"`
	TrackCount int    `json:"track-count"`
}

// mbzRGReleaseList is returned by GET /ws/2/release?release-group=<RG-MBID>&inc=labels+mediums.
type mbzRGReleaseList struct {
	Releases []mbzFullRelease `json:"releases"`
}

// mbzBarcodeSearch is returned by GET /ws/2/release?query=barcode:<barcode>.
type mbzBarcodeSearch struct {
	Releases []mbzFullRelease `json:"releases"`
	Count    int              `json:"count"`
}

// ── MetadataSource ────────────────────────────────────────────────────────────

// FetchGroupContent fetches tracks for a release-group MBID. MusicBrainz
// separates release-groups (the abstract album) from releases (a specific
// pressing), so a two-step lookup is required: first resolve the
// release-group to its canonical release, then fetch track recordings.
// Tracks are flattened across discs; page/perPage slicing is applied locally
// because MusicBrainz does not paginate at the track level.
func (a *Adapter) FetchGroupContent(ctx context.Context, _ domain.ContentType, releaseGroupMBID string, page, perPage int) ([]*domain.ExternalItem, int, error) {
	releaseMBID, err := a.resolveToReleaseMBID(ctx, releaseGroupMBID)
	if err != nil {
		return nil, 0, err
	}

	params := url.Values{}
	params.Set("inc", "recordings")
	params.Set("fmt", "json")
	u := fmt.Sprintf("%srelease/%s?%s", a.baseURL, releaseMBID, params.Encode())

	var release mbzReleaseDetail
	if err := a.get(ctx, u, &release); err != nil {
		return nil, 0, err
	}

	total := 0
	var all []*domain.ExternalItem
	for _, m := range release.Media {
		total += m.TrackCount
		for i := range m.Tracks {
			all = append(all, trackToExternalItem(&m.Tracks[i]))
		}
	}

	offset := (page - 1) * perPage
	if offset >= len(all) {
		return nil, total, nil
	}
	end := offset + perPage
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], total, nil
}

func (a *Adapter) resolveToReleaseMBID(ctx context.Context, releaseGroupMBID string) (string, error) {
	params := url.Values{}
	params.Set("release-group", releaseGroupMBID)
	params.Set("limit", "1")
	params.Set("fmt", "json")
	u := fmt.Sprintf("%srelease?%s", a.baseURL, params.Encode())

	var list mbzReleaseList
	if err := a.get(ctx, u, &list); err != nil {
		return "", fmt.Errorf("resolving release-group %s: %w", releaseGroupMBID, err)
	}
	if len(list.Releases) == 0 {
		return "", fmt.Errorf("musicbrainz: no releases found for release-group %s", releaseGroupMBID)
	}
	return list.Releases[0].ID, nil
}

// FetchReleaseGroupReleases returns all known pressings for a release group.
// The earliest Official release is marked IsDefault; ties broken by MBZ result order.
func (a *Adapter) FetchReleaseGroupReleases(ctx context.Context, rgMBID string) ([]*ports.ExternalMusicRelease, error) {
	params := url.Values{}
	params.Set("release-group", rgMBID)
	params.Set("inc", "labels+mediums")
	params.Set("limit", "100")
	params.Set("fmt", "json")
	u := fmt.Sprintf("%srelease?%s", a.baseURL, params.Encode())

	var list mbzRGReleaseList
	if err := a.get(ctx, u, &list); err != nil {
		return nil, fmt.Errorf("mbz rg releases: %w", err)
	}

	defaultMBID := findDefaultReleaseMBID(list.Releases)

	out := make([]*ports.ExternalMusicRelease, 0, len(list.Releases))
	for i := range list.Releases {
		r := &list.Releases[i]
		rel := toExternalMusicRelease(r, r.ID == defaultMBID)
		out = append(out, rel)
		slog.DebugContext(ctx, "mbz release",
			"mbid", r.ID, "title", r.Title, "country", r.Country,
			"date", r.Date, "barcode", r.Barcode, "is_default", rel.IsDefault)
	}

	slog.InfoContext(ctx, "mbz rg releases fetched",
		"rg_mbid", rgMBID, "count", len(out), "default_mbid", defaultMBID)

	return out, nil
}

// GetReleaseByBarcode resolves a barcode to the matching release via the MBZ
// search API. Returns nil if no release was found.
func (a *Adapter) GetReleaseByBarcode(ctx context.Context, barcode string) (*ports.ExternalMusicRelease, error) {
	params := url.Values{}
	params.Set("query", "barcode:"+barcode)
	params.Set("fmt", "json")
	u := fmt.Sprintf("%srelease?%s", a.baseURL, params.Encode())

	var result mbzBarcodeSearch
	if err := a.get(ctx, u, &result); err != nil {
		slog.InfoContext(ctx, "mbz barcode lookup", "barcode", barcode, "hit", false, "release_mbid", "")
		return nil, fmt.Errorf("mbz barcode lookup: %w", err)
	}

	if len(result.Releases) == 0 {
		slog.InfoContext(ctx, "mbz barcode lookup", "barcode", barcode, "hit", false, "release_mbid", "")
		return nil, ports.ErrNotFound
	}

	r := &result.Releases[0]
	rel := toExternalMusicRelease(r, false)
	slog.InfoContext(ctx, "mbz barcode lookup", "barcode", barcode, "hit", true, "release_mbid", r.ID)
	return rel, nil
}

// findDefaultReleaseMBID returns the MBID of the earliest Official release.
// Empty dates sort last; ties broken by MBZ result order.
func findDefaultReleaseMBID(releases []mbzFullRelease) string {
	best := -1
	for i, r := range releases {
		if r.Status != "Official" {
			continue
		}
		if best == -1 {
			best = i
			continue
		}
		bestDate := releases[best].Date
		if r.Date == "" {
			continue
		}
		if bestDate == "" || r.Date < bestDate {
			best = i
		}
	}
	if best == -1 {
		return ""
	}
	return releases[best].ID
}

// toExternalMusicRelease converts a raw MBZ release to the port transfer type.
func toExternalMusicRelease(r *mbzFullRelease, isDefault bool) *ports.ExternalMusicRelease {
	label := ""
	catalog := ""
	for _, li := range r.LabelInfo {
		if label == "" && li.Label != nil && li.Label.Name != "" {
			label = li.Label.Name
		}
		if catalog == "" && li.CatalogNumber != "" {
			catalog = li.CatalogNumber
		}
		if label != "" && catalog != "" {
			break
		}
	}

	format := ""
	trackCount := 0
	for _, m := range r.Media {
		trackCount += m.TrackCount
		if format == "" {
			format = m.Format
		}
	}

	return &ports.ExternalMusicRelease{
		MBID:          r.ID,
		Title:         r.Title,
		Country:       r.Country,
		Date:          r.Date,
		Label:         label,
		CatalogNumber: catalog,
		Barcode:       r.Barcode,
		Format:        format,
		MediumCount:   len(r.Media),
		TrackCount:    trackCount,
		IsDefault:     isDefault,
	}
}

// ── Mapping ───────────────────────────────────────────────────────────────────

func trackToExternalItem(t *mbzTrack) *domain.ExternalItem {
	title := t.Title
	if title == "" {
		title = t.Recording.Title
	}
	return &domain.ExternalItem{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  t.Recording.ID,
		ContentType: domain.ContentTypeMusic,
		Title:       title,
		Sequence:    t.Number,
		RuntimeSecs: t.Recording.Length / 1000,
	}
}
