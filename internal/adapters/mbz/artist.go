package mbz

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
)

// ── MusicBrainz response types ────────────────────────────────────────────────

type mbzArtistDetail struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Disambiguation string        `json:"disambiguation"`
	Relations      []mbzRelation `json:"relations"`
}

type mbzRelation struct {
	Type string `json:"type"`
	URL  struct {
		Resource string `json:"resource"`
	} `json:"url"`
}

type mbzArtistWithRels struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Relations []mbzArtistRel `json:"relations"`
}

type mbzArtistRel struct {
	Type      string    `json:"type"`
	Direction string    `json:"direction"`
	Artist    mbzArtist `json:"artist"`
}

// mbzArtistEnrichment is the response shape for the full artist enrichment
// request: inc=artist-rels+url-rels+aliases+isnis.
type mbzArtistEnrichment struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Type      string              `json:"type"`
	LifeSpan  mbzLifeSpan         `json:"life-span"`
	BeginArea mbzArea             `json:"begin-area"`
	Relations []mbzEnrichRelation `json:"relations"`
	Aliases   []mbzAlias          `json:"aliases"`
	ISNIs     []string            `json:"isnis"`
}

type mbzLifeSpan struct {
	Begin string `json:"begin"`
	End   string `json:"end"`
}

type mbzArea struct {
	Name string `json:"name"`
}

// mbzEnrichRelation covers both url-rels (URL.Resource non-empty) and
// artist-rels (URL.Resource empty, Direction/Type used for counting members).
type mbzEnrichRelation struct {
	Type      string `json:"type"`
	Direction string `json:"direction"`
	URL       struct {
		Resource string `json:"resource"`
	} `json:"url"`
}

type mbzAlias struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Locale string `json:"locale"`
}

// ── MetadataSource ────────────────────────────────────────────────────────────

// FetchEntryPeople fetches people linked to an artist MBID.
// For group artists it returns band members via "member of band" backward
// relations. For person artists (solo acts) it returns the artist themselves
// so that the solo entry is linked to the matching Person record.
func (a *Adapter) FetchEntryPeople(ctx context.Context, artistMBID string) ([]*domain.ExternalPerson, error) {
	params := url.Values{}
	params.Set("inc", "artist-rels")
	params.Set("fmt", "json")
	u := fmt.Sprintf("%sartist/%s?%s", a.baseURL, artistMBID, params.Encode())

	var artist mbzArtistWithRels
	if err := a.get(ctx, u, &artist); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, ports.ErrNotFound
		}
		return nil, err
	}

	var members []*domain.ExternalPerson
	for _, rel := range artist.Relations {
		if rel.Type != "member of band" || rel.Direction != "backward" {
			continue
		}
		members = append(members, &domain.ExternalPerson{
			Source:     domain.SourceMusicBrainz,
			ExternalID: rel.Artist.ID,
			Name:       rel.Artist.Name,
			Role:       domain.RoleArtist,
		})
	}

	// A solo artist in MusicBrainz is typed "Person" — the artist record IS the
	// person. Link the artist to themselves so PersonDetail shows the solo entry
	// under "Member of" and the artist page shows the person in its members list.
	if artist.Type == "Person" && len(members) == 0 {
		members = append(members, &domain.ExternalPerson{
			Source:     domain.SourceMusicBrainz,
			ExternalID: artist.ID,
			Name:       artist.Name,
			Role:       domain.RoleArtist,
		})
	}

	return members, nil
}

// FetchEntryMetadata fetches enrichment metadata for a music artist MBID.
// For group artists the keys founded_date, founded_location, and dissolved_date
// are populated from the life-span. For solo artists (type "Person") the
// equivalent born_date, born_location, and died_date keys are used instead.
func (a *Adapter) FetchEntryMetadata(ctx context.Context, _ domain.ContentType, mbid string) (map[string]any, error) {
	params := url.Values{}
	params.Set("inc", "artist-rels+url-rels+aliases")
	params.Set("fmt", "json")
	u := fmt.Sprintf("%sartist/%s?%s", a.baseURL, mbid, params.Encode())

	var artist mbzArtistEnrichment
	if err := a.get(ctx, u, &artist); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, ports.ErrNotFound
		}
		return nil, err
	}

	meta := make(map[string]any)
	enrichArtistType(meta, mbid, artist.Type)
	enrichLifeSpan(meta, artist.Type, artist.LifeSpan, artist.BeginArea)
	if len(artist.ISNIs) > 0 {
		meta["isni"] = artist.ISNIs[0]
	}

	aliases := filterAliases(artist.Aliases)
	if len(aliases) > 0 {
		meta["aliases"] = aliases
	}

	memberCount, urlCount := enrichURLRelations(meta, artist.Relations)

	slog.Info("mbz artist enriched",
		"mbid", mbid,
		"name", artist.Name,
		"type", artist.Type,
		"aliases", len(aliases),
		"members", memberCount,
		"url_relations", urlCount,
	)

	return meta, nil
}

func enrichArtistType(meta map[string]any, mbid, artistType string) {
	if artistType == "" {
		return
	}
	switch artistType {
	case "Group", "Person", "Orchestra", "Choir", "Character", "Other":
	default:
		slog.Warn("unknown artist type", "mbid", mbid, "type", artistType)
	}
	meta["artist_type"] = strings.ToLower(artistType)
}

func enrichLifeSpan(meta map[string]any, artistType string, ls mbzLifeSpan, area mbzArea) {
	if artistType == "Person" {
		if ls.Begin != "" {
			meta["born_date"] = ls.Begin
		}
		if ls.End != "" {
			meta["died_date"] = ls.End
		}
		if area.Name != "" {
			meta["born_location"] = area.Name
		}
		return
	}
	if ls.Begin != "" {
		meta["founded_date"] = ls.Begin
	}
	if ls.End != "" {
		meta["dissolved_date"] = ls.End
	}
	if area.Name != "" {
		meta["founded_location"] = area.Name
	}
}

func enrichURLRelations(meta map[string]any, relations []mbzEnrichRelation) (memberCount, urlCount int) {
	for _, rel := range relations {
		if rel.Type == "member of band" && rel.Direction == "backward" {
			memberCount++
			continue
		}
		if rel.URL.Resource == "" {
			continue
		}
		urlCount++
		slog.Debug("mbz url relation", "type", rel.Type, "url", rel.URL.Resource)
		switch rel.Type {
		case "official homepage":
			meta["official_url"] = rel.URL.Resource
		case "last.fm":
			meta["lastfm_url"] = rel.URL.Resource
		case "wikipedia":
			meta["wikipedia_url"] = rel.URL.Resource
		}
	}
	return
}

func filterAliases(aliases []mbzAlias) []string {
	var out []string
	for _, a := range aliases {
		if a.Locale != "" && a.Locale != "en" {
			continue
		}
		if a.Type != "" && a.Type != "Artist name" && a.Type != "Search hint" {
			continue
		}
		out = append(out, a.Name)
	}
	return out
}

// FindByExternalID fetches a single artist by MBID.
// Returns ports.ErrNotFound when the MBID does not exist in MusicBrainz.
func (a *Adapter) FindByExternalID(ctx context.Context, _ domain.ContentType, id string) (*domain.ExternalItem, error) {
	params := url.Values{}
	params.Set("inc", "url-rels")
	params.Set("fmt", "json")
	u := fmt.Sprintf("%sartist/%s?%s", a.baseURL, id, params.Encode())

	var artist mbzArtistDetail
	if err := a.get(ctx, u, &artist); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, ports.ErrNotFound
		}
		return nil, err
	}
	return &domain.ExternalItem{
		Source:      domain.SourceMusicBrainz,
		ExternalID:  artist.ID,
		ContentType: domain.ContentTypeMusic,
		Title:       artist.Name,
		Overview:    artist.Disambiguation,
		ExternalIDs: map[string]string{"mbid": artist.ID},
	}, nil
}
