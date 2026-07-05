package mbz

import (
	"context"
	"fmt"
	"net/url"
	"purser/internal/domain"
	"strconv"
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
		out[i] = toExternalRecording(&resp.Recordings[i])
	}
	return out, nil
}

// ── Mapping ───────────────────────────────────────────────────────────────────

// FetchRecordingByID fetches a single recording from MusicBrainz by its MBID,
// including artist credits. Used by the music identifier for direct MBID lookups.
func (a *Adapter) FetchRecordingByID(ctx context.Context, mbid string) (*domain.ExternalItem, error) {
	u := fmt.Sprintf("%srecording/%s?inc=artist-credits+releases&fmt=json", a.baseURL, mbid)
	var r mbzRecording
	if err := a.get(ctx, u, &r); err != nil {
		return nil, err
	}
	if r.ID == "" {
		return nil, fmt.Errorf("mbz recording not found: %s", mbid)
	}
	return toExternalRecording(&r), nil
}

// FindItemByExternalID fetches a recording by MBID. Implements ports.ItemSource.
func (a *Adapter) FindItemByExternalID(ctx context.Context, _ domain.ContentType, id string) (*domain.ExternalItem, error) {
	return a.FetchRecordingByID(ctx, id)
}

func toExternalRecording(r *mbzRecording) *domain.ExternalItem {
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
	// Use the first release to populate the group external ID and album cover.
	// Cover Art Archive URLs are deterministic so no HTTP fetch is needed at this stage.
	if len(r.Releases) > 0 && r.Releases[0].ID != "" {
		item.GroupExternalID = r.Releases[0].ID
		item.GroupTitle = r.Releases[0].Title
		item.ImageURL = "https://coverartarchive.org/release/" + r.Releases[0].ID + "/front-250"
	}
	return item
}
