package stashdb

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
)

// ── GraphQL response types ────────────────────────────────────────────────────

type gqlStudio struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	URLs   []gqlURL      `json:"urls"`
	Images []gqlImage    `json:"images"`
	Parent *gqlStudioRef `json:"parent"`
}

type gqlStudioRef struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Images []gqlImage `json:"images"`
	URLs   []gqlURL   `json:"urls"`
}

type gqlImage struct {
	URL string `json:"url"`
}

type gqlURL struct {
	URL  string `json:"url"`
	Type string `json:"type"`
}

// ── Queries ───────────────────────────────────────────────────────────────────

const searchStudiosQuery = `
query SearchStudios($term: String!) {
  searchStudio(term: $term) {
    id
    name
    urls { url type }
    images { url }
    parent { id name images { url } urls { url type } }
  }
}`

const findStudioByIDQuery = `
query FindStudio($id: ID!) {
  findStudio(id: $id) {
    id name images { url }
  }
}`

// ── MetadataSource ────────────────────────────────────────────────────────────

// findStudioByID fetches a studio by its StashDB ID and returns an ExternalItem
// carrying the studio's images. Returns ports.ErrNotFound when the ID is not a studio.
func (a *Adapter) findStudioByID(ctx context.Context, id string) (*domain.ExternalItem, error) {
	var resp struct {
		FindStudio *gqlStudio `json:"findStudio"`
	}
	if err := a.gql(ctx, findStudioByIDQuery, map[string]any{"id": id}, &resp); err != nil {
		return nil, err
	}
	if resp.FindStudio == nil {
		return nil, ports.ErrNotFound
	}
	s := resp.FindStudio
	images := make([]domain.ExternalImage, 0, len(s.Images))
	for _, img := range s.Images {
		if img.URL != "" {
			images = append(images, domain.ExternalImage{Type: domain.ImageTypePoster, URL: img.URL})
		}
	}
	return &domain.ExternalItem{
		Source:     domain.SourceStashDB,
		ExternalID: s.ID,
		Title:      s.Name,
		Images:     images,
	}, nil
}

// SearchStudios queries StashDB for studios matching the given search string.
func (a *Adapter) SearchStudios(ctx context.Context, query string, limit int) ([]*domain.ExternalStudio, error) {
	var resp struct {
		SearchStudio []gqlStudio `json:"searchStudio"`
	}
	if err := a.gql(ctx, searchStudiosQuery, map[string]any{"term": query}, &resp); err != nil {
		return nil, err
	}

	results := resp.SearchStudio
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	out := make([]*domain.ExternalStudio, len(results))
	for i := range results {
		out[i] = toExternalStudio(&results[i])
	}
	return out, nil
}

// ── Mapping ───────────────────────────────────────────────────────────────────

func toExternalStudio(s *gqlStudio) *domain.ExternalStudio {
	e := &domain.ExternalStudio{
		Source:     domain.SourceStashDB,
		ExternalID: s.ID,
		Name:       s.Name,
	}
	if len(s.Images) > 0 {
		e.ImageURL = s.Images[0].URL
	}
	for _, u := range s.URLs {
		if u.Type == "HOME" || e.WebsiteURL == "" {
			e.WebsiteURL = u.URL
		}
	}
	if s.Parent != nil {
		e.ParentID = s.Parent.ID
		e.ParentName = s.Parent.Name
		if len(s.Parent.Images) > 0 {
			e.ParentImageURL = s.Parent.Images[0].URL
		}
		for _, u := range s.Parent.URLs {
			if u.Type == "HOME" || e.ParentWebsiteURL == "" {
				e.ParentWebsiteURL = u.URL
			}
		}
	}
	return e
}
