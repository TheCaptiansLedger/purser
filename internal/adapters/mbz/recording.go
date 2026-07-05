package mbz

import (
	"context"
	"fmt"
	"net/url"
	"purser/internal/domain"
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
}

type mbzArtistCredit struct {
	Name   string `json:"name"`
	Artist struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
}

// ── MetadataSource ────────────────────────────────────────────────────────────

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
		out[i] = toExternalRecording(&resp.Recordings[i], "")
	}
	return out, nil
}

// ── Mapping ───────────────────────────────────────────────────────────────────

// FetchRecordingByID fetches a single recording from MusicBrainz by its MBID,
// including artist credits. albumHint is matched case-insensitively against
// the recording's release titles to select the best release for cover art and
// grouping; pass empty string to use the first release MBZ returns.
func (a *Adapter) FetchRecordingByID(ctx context.Context, mbid, albumHint string) (*domain.ExternalItem, error) {
	u := fmt.Sprintf("%srecording/%s?inc=artist-credits+releases&fmt=json", a.baseURL, mbid)
	var r mbzRecording
	if err := a.get(ctx, u, &r); err != nil {
		return nil, err
	}
	if r.ID == "" {
		return nil, fmt.Errorf("mbz recording not found: %s", mbid)
	}
	return toExternalRecording(&r, albumHint), nil
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

func toExternalRecording(r *mbzRecording, albumHint string) *domain.ExternalItem {
	item := &domain.ExternalItem{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  r.ID,
		ContentType: domain.ContentTypeMusic,
		Title:       r.Title,
		RuntimeSecs: r.Length / 1000,
	}
	// Top-level ArtistCredit is present on direct lookup; fall back to releases for search results.
	var credits []mbzArtistCredit
	if len(r.ArtistCredit) > 0 {
		credits = r.ArtistCredit
	} else if len(r.Releases) > 0 {
		credits = r.Releases[0].ArtistCredit
	}
	if len(credits) > 0 {
		ac := &credits[0]
		item.Studio = &domain.ExternalStudio{
			Source:     domain.SourceMusicBrainz,
			ExternalID: ac.Artist.ID,
			Name:       ac.Name,
		}
	}
	// Prefer the release whose title matches albumHint (embedded album tag); fall
	// back to first. Cover Art Archive URLs are deterministic — no HTTP fetch needed.
	if rel := pickRelease(r.Releases, albumHint); rel != nil {
		item.GroupExternalID = rel.ID
		item.GroupTitle = rel.Title
		item.ImageURL = "https://coverartarchive.org/release/" + rel.ID + "/front-250"
	}
	return item
}
