package stashdb

import (
	"context"

	"purser/internal/domain"
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

// ── MetadataSource ────────────────────────────────────────────────────────────

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
