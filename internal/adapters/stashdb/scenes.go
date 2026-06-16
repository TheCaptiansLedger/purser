package stashdb

import (
	"context"
	"time"

	"purser/internal/domain"
	"purser/internal/ports"
)

// ── GraphQL response types ────────────────────────────────────────────────────

type gqlScene struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"details"`  // StashDB uses "details" not "description"
	Date        string     `json:"date"`      // "YYYY-MM-DD"
	Duration    int        `json:"duration"`  // seconds
	Images      []gqlImage `json:"images"`
	Tags        []struct {
		Name string `json:"name"`
	} `json:"tags"`
	Studio *struct {
		ID     string        `json:"id"`
		Name   string        `json:"name"`
		Parent *gqlStudioRef `json:"parent"`
	} `json:"studio"`
	Performers []struct {
		Performer gqlPerformer `json:"performer"`
	} `json:"performers"`
}

// ── Queries ───────────────────────────────────────────────────────────────────

// sceneFields is the common field set requested on every scene query.
const sceneFields = `
    id title details date duration
    images { url }
    tags { name }
    studio { id name parent { id name } }
    performers {
      performer {
        id name aliases images { url }
        birthdate { date }
        height hair_color eye_color
        tattoos { location description }
        piercings { location description }
        career_start_year
        measurements { cup_size band_size waist hip }
      }
    }`

const fetchStudioScenesQuery = `
query FetchStudioScenes($id: ID!, $page: Int!, $perPage: Int!) {
  queryScenes(input: {
    studios: { value: [$id], modifier: INCLUDES }
    per_page: $perPage
    page: $page
    sort: DATE
    direction: DESC
  }) {
    count
    scenes {` + sceneFields + `
    }
  }
}`

const searchScenesQuery = `
query SearchScenes($title: String!, $limit: Int!) {
  queryScenes(input: {
    title: $title
    per_page: $limit
    sort: DATE
    direction: DESC
  }) {
    scenes {` + sceneFields + `
    }
  }
}`

const findSceneByIDQuery = `
query FindScene($id: ID!) {
  findScene(id: $id) {` + sceneFields + `
  }
}`

// StashDB uses OSHash as its primary fingerprint algorithm for video files.
const findScenesByFingerprintQuery = `
query FindScenesByFingerprints($fingerprints: [FingerprintQueryInput!]!) {
  findScenesByFingerprints(fingerprints: $fingerprints) {` + sceneFields + `
  }
}`

// ── MetadataSource ────────────────────────────────────────────────────────────

func (a *Adapter) SearchItems(ctx context.Context, contentType domain.ContentType, query string, limit int) ([]*domain.ExternalItem, error) {
	var resp struct {
		QueryScenes struct {
			Scenes []gqlScene `json:"scenes"`
		} `json:"queryScenes"`
	}
	if err := a.gql(ctx, searchScenesQuery, map[string]any{"title": query, "limit": limit}, &resp); err != nil {
		return nil, err
	}

	scenes := resp.QueryScenes.Scenes
	out := make([]*domain.ExternalItem, len(scenes))
	for i := range scenes {
		out[i] = toExternalItem(&scenes[i], contentType)
	}
	return out, nil
}

// FindByHash looks up a scene by OSHash. Returns nil, nil when no match is found.
func (a *Adapter) FindByHash(ctx context.Context, hash string) (*domain.ExternalItem, error) {
	var resp struct {
		FindScenesByFingerprints []gqlScene `json:"findScenesByFingerprints"`
	}
	vars := map[string]any{
		"fingerprints": []map[string]any{
			{"hash": hash, "algorithm": "OSHASH"},
		},
	}
	if err := a.gql(ctx, findScenesByFingerprintQuery, vars, &resp); err != nil {
		return nil, err
	}
	if len(resp.FindScenesByFingerprints) == 0 {
		return nil, nil
	}
	return toExternalItem(&resp.FindScenesByFingerprints[0], domain.ContentTypeAdult), nil
}

// FindByExternalID fetches a single scene by its StashDB ID.
// Returns nil, nil when the ID is not found.
func (a *Adapter) FindByExternalID(ctx context.Context, id string) (*domain.ExternalItem, error) {
	var resp struct {
		FindScene *gqlScene `json:"findScene"`
	}
	if err := a.gql(ctx, findSceneByIDQuery, map[string]any{"id": id}, &resp); err != nil {
		return nil, err
	}
	if resp.FindScene == nil {
		return nil, nil
	}
	return toExternalItem(resp.FindScene, domain.ContentTypeAdult), nil
}

// FetchEntryContent pages through all scenes for a studio. StashDB scenes are
// flat — groups is always nil; items contains the page of scenes; total is the
// scene count across all pages.
func (a *Adapter) FetchEntryContent(ctx context.Context, externalID string, page, perPage int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	var resp struct {
		QueryScenes struct {
			Count  int        `json:"count"`
			Scenes []gqlScene `json:"scenes"`
		} `json:"queryScenes"`
	}
	vars := map[string]any{
		"id":      externalID,
		"page":    page,
		"perPage": perPage,
	}
	if err := a.gql(ctx, fetchStudioScenesQuery, vars, &resp); err != nil {
		return nil, nil, 0, err
	}
	scenes := resp.QueryScenes.Scenes
	items := make([]*domain.ExternalItem, len(scenes))
	for i := range scenes {
		items[i] = toExternalItem(&scenes[i], domain.ContentTypeAdult)
	}
	return nil, items, resp.QueryScenes.Count, nil
}

// FetchGroupContent returns ErrNotSupported — StashDB scenes are flat; there is
// no intermediate group layer between a studio and its scenes.
func (a *Adapter) FetchGroupContent(_ context.Context, _ string, _, _ int) ([]*domain.ExternalItem, int, error) {
	return nil, 0, ports.ErrNotSupported
}

// ── Mapping ───────────────────────────────────────────────────────────────────

func toExternalItem(s *gqlScene, contentType domain.ContentType) *domain.ExternalItem {
	e := &domain.ExternalItem{
		Source:      domain.SourceStashDB,
		ExternalID:  s.ID,
		ContentType: contentType,
		Title:       s.Title,
		Overview:    s.Description,
		RuntimeSecs: s.Duration,
	}

	if s.Date != "" {
		if t, err := time.Parse("2006-01-02", s.Date); err == nil {
			e.Date = t
		}
	}
	if len(s.Images) > 0 {
		e.ImageURL = s.Images[0].URL
	}
	for _, t := range s.Tags {
		e.Tags = append(e.Tags, t.Name)
	}
	if s.Studio != nil {
		e.Studio = &domain.ExternalStudio{
			Source:     domain.SourceStashDB,
			ExternalID: s.Studio.ID,
			Name:       s.Studio.Name,
		}
		if s.Studio.Parent != nil {
			e.Studio.ParentID = s.Studio.Parent.ID
			e.Studio.ParentName = s.Studio.Parent.Name
		}
	}
	for i := range s.Performers {
		e.People = append(e.People, toExternalPerson(&s.Performers[i].Performer))
	}
	return e
}
