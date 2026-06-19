package mbz

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"purser/internal/domain"
	"strconv"
)

// ── MusicBrainz response types ────────────────────────────────────────────────

type mbzArtistResponse struct {
	Artists []mbzArtist `json:"artists"`
}

type mbzArtist struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Disambiguation string `json:"disambiguation"`
	Country        string `json:"country"`
}

// ── MetadataSource ────────────────────────────────────────────────────────────

// SearchStudios queries MusicBrainz for artists by name. No type filter is applied
// so both groups (Fleetwood Mac) and solo artists (Stevie Nicks) are returned.
func (a *Adapter) SearchStudios(ctx context.Context, query string, limit int) ([]*domain.ExternalStudio, error) {
	u := a.artistURL(query, "", limit)
	slog.Debug("mbz: SearchStudios", "query", query, "url", u)
	var resp mbzArtistResponse
	if err := a.get(ctx, u, &resp); err != nil {
		slog.Warn("mbz: SearchStudios failed", "query", query, "error", err)
		return nil, err
	}
	slog.Debug("mbz: SearchStudios result", "query", query, "count", len(resp.Artists))
	out := make([]*domain.ExternalStudio, len(resp.Artists))
	for i := range resp.Artists {
		out[i] = artistToStudio(&resp.Artists[i])
	}
	return out, nil
}

// SearchPeople queries MusicBrainz for individual artists (type:Person) by name.
func (a *Adapter) SearchPeople(ctx context.Context, query string, limit int) ([]*domain.ExternalPerson, error) {
	u := a.artistURL(query, "Person", limit)
	slog.Debug("mbz: SearchPeople", "query", query, "url", u)
	var resp mbzArtistResponse
	if err := a.get(ctx, u, &resp); err != nil {
		slog.Warn("mbz: SearchPeople failed", "query", query, "error", err)
		return nil, err
	}
	slog.Debug("mbz: SearchPeople result", "query", query, "count", len(resp.Artists))
	out := make([]*domain.ExternalPerson, len(resp.Artists))
	for i := range resp.Artists {
		out[i] = artistToPerson(&resp.Artists[i])
	}
	return out, nil
}

// ── URL building ──────────────────────────────────────────────────────────────

func (a *Adapter) artistURL(query, artistType string, limit int) string {
	params := url.Values{}
	if artistType != "" {
		params.Set("query", query+" AND type:"+artistType)
	} else {
		params.Set("query", query)
	}
	params.Set("fmt", "json")
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	return fmt.Sprintf("%sartist?%s", a.baseURL, params.Encode())
}

// ── Mapping ───────────────────────────────────────────────────────────────────

func artistToStudio(ar *mbzArtist) *domain.ExternalStudio {
	return &domain.ExternalStudio{
		Source:     domain.SourceMusicBrainz,
		ExternalID: ar.ID,
		Name:       ar.Name,
		Overview:   ar.Disambiguation,
	}
}

func artistToPerson(ar *mbzArtist) *domain.ExternalPerson {
	e := &domain.ExternalPerson{
		Source:     domain.SourceMusicBrainz,
		ExternalID: ar.ID,
		Name:       ar.Name,
		Overview:   ar.Disambiguation,
		Role:       domain.RoleArtist,
	}
	if ar.Country != "" {
		e.Metadata = map[string]any{"nationality": ar.Country}
	}
	return e
}
