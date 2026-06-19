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
	ID       string                `json:"id"`
	Title    string                `json:"title"`
	Length   int                   `json:"length"` // milliseconds
	Releases []mbzRecordingRelease `json:"releases"`
}

type mbzRecordingRelease struct {
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

func toExternalRecording(r *mbzRecording) *domain.ExternalItem {
	item := &domain.ExternalItem{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  r.ID,
		ContentType: domain.ContentTypeMusic,
		Title:       r.Title,
		RuntimeSecs: r.Length / 1000,
	}
	if len(r.Releases) > 0 && len(r.Releases[0].ArtistCredit) > 0 {
		ac := &r.Releases[0].ArtistCredit[0]
		item.Studio = &domain.ExternalStudio{
			Source:     domain.SourceMusicBrainz,
			ExternalID: ac.Artist.ID,
			Name:       ac.Name,
		}
	}
	return item
}
