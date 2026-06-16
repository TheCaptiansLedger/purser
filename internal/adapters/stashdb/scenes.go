package stashdb

import (
	"context"
	"time"

	"purser/internal/domain"
)

// ── GraphQL response types ────────────────────────────────────────────────────

type gqlScene struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Date        string     `json:"date"` // "YYYY-MM-DD"
	Duration    int        `json:"duration"` // seconds
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
    id title description date duration
    images { url }
    tags { name }
    studio { id name parent { id name } }
    performers {
      performer {
        id name aliases images { url }
        birthdate { date }
        height hair_color eye_color tattoos piercings career_start_year
        measurements { cup_size band_size waist hip }
      }
    }`

const searchScenesQuery = `
query SearchScenes($title: String!, $limit: Int!) {
  queryScenes(input: {
    title: $title
    per_page: $limit
    sort: "date"
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
