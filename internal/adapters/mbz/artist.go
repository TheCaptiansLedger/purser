package mbz

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"purser/internal/domain"
	"purser/internal/ports"
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
	}, nil
}
