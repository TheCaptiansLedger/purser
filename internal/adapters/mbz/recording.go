package mbz

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"purser/internal/domain"
	"purser/internal/ports"
	"strconv"
	"strings"
)

// ── MusicBrainz response types ────────────────────────────────────────────────

type mbzRecordingResponse struct {
	Recordings []mbzRecording `json:"recordings"`
	Count      int            `json:"count"`
}

type mbzRecording struct {
	ID           string                `json:"id"`
	Title        string                `json:"title"`
	Length       int                   `json:"length"`        // milliseconds
	ArtistCredit []mbzArtistCredit     `json:"artist-credit"` // present on direct lookup
	Releases     []mbzRecordingRelease `json:"releases"`
}

type mbzRecordingRelease struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	ArtistCredit []mbzArtistCredit `json:"artist-credit"`
	ReleaseGroup mbzReleaseGroup   `json:"release-group"`
	LabelInfo    []mbzLabelInfo    `json:"label-info"`
	Barcode      string            `json:"barcode"`
	Country      string            `json:"country"`
	Date         string            `json:"date"`
}

type mbzLabelInfo struct {
	Label         *mbzLabel `json:"label"`
	CatalogNumber string    `json:"catalog-number"`
}

type mbzLabel struct {
	Name string `json:"name"`
}

type mbzArtistCredit struct {
	Name   string `json:"name"`
	Artist struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
}

// ── MetadataSource ────────────────────────────────────────────────────────────

// FindByHash is not supported by MusicBrainz; it returns ErrNotSupported.
func (a *Adapter) FindByHash(_ context.Context, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
}

// SearchItems searches MusicBrainz for recordings matching the given title.
func (a *Adapter) SearchItems(ctx context.Context, _ domain.ContentType, query string, limit int) ([]*domain.ExternalItem, error) {
	params := url.Values{}
	params.Set("query", "recording:"+query)
	params.Set("fmt", "json")
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	u := fmt.Sprintf("%srecording?%s", a.baseURL, params.Encode())

	var resp mbzRecordingResponse
	if err := a.get(ctx, u, &resp); err != nil {
		return nil, err
	}
	out := make([]*domain.ExternalItem, len(resp.Recordings))
	for i := range resp.Recordings {
		out[i] = toExternalRecording(ctx, &resp.Recordings[i], "")
	}
	return out, nil
}

// ── Mapping ───────────────────────────────────────────────────────────────────

// FetchRecordingByID fetches a single recording from MusicBrainz by its MBID,
// including artist credits and release group IDs. albumHint is matched
// case-insensitively against the recording's release titles to select the best
// release for cover art and grouping; pass empty string to use the first release
// MBZ returns.
func (a *Adapter) FetchRecordingByID(ctx context.Context, mbid, albumHint string) (*domain.ExternalItem, error) {
	u := fmt.Sprintf("%srecording/%s?inc=artist-credits+releases+release-groups&fmt=json", a.baseURL, mbid)
	var r mbzRecording
	if err := a.get(ctx, u, &r); err != nil {
		return nil, err
	}
	if r.ID == "" {
		return nil, fmt.Errorf("mbz recording not found: %s", mbid)
	}
	item := toExternalRecording(ctx, &r, albumHint)
	selectedReleaseMBID := ""
	selectedReleaseTitle := ""
	rgMBID := ""
	if item.ReleaseDetail != nil {
		selectedReleaseMBID = item.ReleaseDetail.ReleaseMBID
		selectedReleaseTitle = item.ReleaseDetail.ReleaseTitle
		rgMBID = item.GroupExternalID
	}
	slog.DebugContext(ctx, "mbz: recording fetched",
		"recording_mbid", mbid,
		"recording_title", r.Title,
		"release_count", len(r.Releases),
		"album_hint", albumHint,
		"selected_release_mbid", selectedReleaseMBID,
		"selected_release_title", selectedReleaseTitle,
		"release_group_mbid", rgMBID,
	)
	return item, nil
}

// FindItemByExternalID fetches a recording by MBID. Implements ports.ItemSource.
func (a *Adapter) FindItemByExternalID(ctx context.Context, _ domain.ContentType, id string) (*domain.ExternalItem, error) {
	return a.FetchRecordingByID(ctx, id, "")
}

// pickRelease returns the release from candidates that best matches albumHint
// (case-insensitive substring), falling back to the first release.
func pickRelease(releases []mbzRecordingRelease, albumHint string) *mbzRecordingRelease {
	if len(releases) == 0 {
		return nil
	}
	if albumHint != "" {
		hint := strings.ToLower(albumHint)
		for i := range releases {
			if strings.Contains(strings.ToLower(releases[i].Title), hint) || strings.Contains(hint, strings.ToLower(releases[i].Title)) {
				return &releases[i]
			}
		}
	}
	return &releases[0]
}

func toExternalRecording(ctx context.Context, r *mbzRecording, albumHint string) *domain.ExternalItem {
	// Prefer the release whose title matches albumHint (embedded album tag); fall
	// back to first. GroupExternalID is the release group MBID (the conceptual album),
	// not the release MBID (the specific pressing), so scanned files can be matched to
	// albums imported via discography which also uses release group MBIDs.
	rel := pickRelease(r.Releases, albumHint)
	// A release without an ID is a stub (e.g. embedded in a search response for
	// artist-credit extraction only). Treat it as absent so we don't set an empty
	// GroupExternalID or emit a misleading WARN.
	if rel == nil || rel.ID == "" {
		item := &domain.ExternalItem{
			Source:      domain.SourceMusicBrainz,
			ExternalID:  r.ID,
			ContentType: domain.ContentTypeMusic,
			Title:       r.Title,
			RuntimeSecs: r.Length / 1000,
		}
		var credits []mbzArtistCredit
		if len(r.ArtistCredit) > 0 {
			credits = r.ArtistCredit
		} else if len(r.Releases) > 0 {
			credits = r.Releases[0].ArtistCredit
		}
		if len(credits) > 0 {
			ac := &credits[0]
			item.Studio = &domain.ExternalStudio{Source: domain.SourceMusicBrainz, ExternalID: ac.Artist.ID, Name: ac.Name}
		}
		return item
	}
	if rel.ReleaseGroup.ID == "" {
		slog.WarnContext(ctx, "mbz: release has no release-group",
			"recording_mbid", r.ID,
			"release_mbid", rel.ID,
			"release_title", rel.Title,
		)
	}
	return toExternalItemForRelease(r, rel)
}

// toExternalItemForRelease builds one ExternalItem for a specific release of a recording.
// r provides recording-level fields; rel provides the edition-specific fields.
func toExternalItemForRelease(r *mbzRecording, rel *mbzRecordingRelease) *domain.ExternalItem {
	item := &domain.ExternalItem{
		Source:          domain.SourceMusicBrainz,
		ExternalID:      r.ID,
		ContentType:     domain.ContentTypeMusic,
		Title:           r.Title,
		RuntimeSecs:     r.Length / 1000,
		GroupExternalID: rel.ReleaseGroup.ID,
		GroupTitle:      rel.ReleaseGroup.Title,
		ImageURL:        "https://coverartarchive.org/release/" + rel.ID + "/front-250",
		ReleaseDetail: &domain.ExternalReleaseDetail{
			ReleaseMBID:    rel.ID,
			ReleaseTitle:   rel.Title,
			ReleaseDate:    rel.Date,
			ReleaseLabel:   releaseLabel(rel),
			ReleaseCountry: rel.Country,
			ReleaseCatalog: releaseCatalog(rel),
			ReleaseBarcode: rel.Barcode,
		},
	}
	credits := r.ArtistCredit
	if len(credits) == 0 {
		credits = rel.ArtistCredit
	}
	if len(credits) > 0 {
		ac := &credits[0]
		item.Studio = &domain.ExternalStudio{
			Source:     domain.SourceMusicBrainz,
			ExternalID: ac.Artist.ID,
			Name:       ac.Name,
		}
	}
	return item
}

// FetchAllRecordingReleases fetches all releases for a recording and returns one
// ExternalItem per release. Releases without a release-group ID are skipped with
// a WARN log. Used by the multi-candidate identifier strategy.
func (a *Adapter) FetchAllRecordingReleases(ctx context.Context, mbid string) ([]*domain.ExternalItem, error) {
	u := fmt.Sprintf("%srecording/%s?inc=artist-credits+releases+release-groups&fmt=json", a.baseURL, mbid)
	var r mbzRecording
	if err := a.get(ctx, u, &r); err != nil {
		return nil, err
	}
	if r.ID == "" {
		return nil, fmt.Errorf("mbz recording not found: %s", mbid)
	}
	items := make([]*domain.ExternalItem, 0, len(r.Releases))
	for i := range r.Releases {
		rel := &r.Releases[i]
		if rel.ID == "" {
			continue
		}
		if rel.ReleaseGroup.ID == "" {
			slog.WarnContext(ctx, "mbz: release has no release-group",
				"recording_mbid", mbid,
				"release_mbid", rel.ID,
				"release_title", rel.Title,
			)
			continue
		}
		items = append(items, toExternalItemForRelease(&r, rel))
	}
	return items, nil
}

func releaseLabel(rel *mbzRecordingRelease) string {
	for _, li := range rel.LabelInfo {
		if li.Label != nil && li.Label.Name != "" {
			return li.Label.Name
		}
	}
	return ""
}

func releaseCatalog(rel *mbzRecordingRelease) string {
	for _, li := range rel.LabelInfo {
		if li.CatalogNumber != "" {
			return li.CatalogNumber
		}
	}
	return ""
}
