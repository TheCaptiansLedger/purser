package stashdb

import (
	"context"
	"purser/internal/domain"
	"purser/internal/ports"
	"sync"
	"time"
)

// ── GraphQL response types ────────────────────────────────────────────────────

type gqlScene struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"details"`  // StashDB uses "details" not "description"
	Date        string     `json:"date"`     // "YYYY-MM-DD"
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
    id title details date duration
    images { url }
    tags { name }
    studio { id name parent { id name } }
    performers {
      performer {
        id name aliases images { url }
        birthdate { date }
        height hair_color eye_color
        gender ethnicity country breast_type
        career_start_year career_end_year disambiguation
        tattoos { location description }
        piercings { location description }
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
query FindScenesByFingerprints($fingerprints: [[FingerprintQueryInput!]!]!) {
  findScenesBySceneFingerprints(fingerprints: $fingerprints) {` + sceneFields + `
  }
}`

// ── MetadataSource ────────────────────────────────────────────────────────────

// SearchItems queries StashDB for scenes matching the given search string.
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

// FindByHash looks up a scene by OSHash. Returns ports.ErrNotFound when no match exists.
func (a *Adapter) FindByHash(ctx context.Context, hash string) (*domain.ExternalItem, error) {
	var resp struct {
		FindScenesBySceneFingerprints [][]gqlScene `json:"findScenesBySceneFingerprints"`
	}
	vars := map[string]any{
		"fingerprints": [][]map[string]any{
			{{"hash": hash, "algorithm": "OSHASH"}},
		},
	}
	if err := a.gql(ctx, findScenesByFingerprintQuery, vars, &resp); err != nil {
		return nil, err
	}
	if len(resp.FindScenesBySceneFingerprints) == 0 || len(resp.FindScenesBySceneFingerprints[0]) == 0 {
		return nil, ports.ErrNotFound
	}
	return toExternalItem(&resp.FindScenesBySceneFingerprints[0][0], domain.ContentTypeAdult), nil
}

// FindByExternalID resolves a StashDB UUID to its entity — scene, performer, or
// studio — by querying all three endpoints in parallel. This is necessary because
// the same external-ID slot is used for entries (studios), items (scenes), and
// people (performers), and the UUID namespace is flat within StashDB.
func (a *Adapter) FindByExternalID(ctx context.Context, _ domain.ContentType, id string) (*domain.ExternalItem, error) {
	type result struct {
		item *domain.ExternalItem
		err  error
	}

	sceneCh := make(chan result, 1)
	perfCh := make(chan result, 1)
	studioCh := make(chan result, 1)

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		item, err := a.findSceneByID(ctx, id)
		sceneCh <- result{item, err}
	}()
	go func() {
		defer wg.Done()
		item, err := a.findPerformerByID(ctx, id)
		perfCh <- result{item, err}
	}()
	go func() {
		defer wg.Done()
		item, err := a.findStudioByID(ctx, id)
		studioCh <- result{item, err}
	}()
	wg.Wait()

	if r := <-sceneCh; r.item != nil {
		return r.item, nil
	}
	if r := <-perfCh; r.item != nil {
		return r.item, nil
	}
	if r := <-studioCh; r.item != nil {
		return r.item, nil
	}
	return nil, ports.ErrNotFound
}

func (a *Adapter) findSceneByID(ctx context.Context, id string) (*domain.ExternalItem, error) {
	var resp struct {
		FindScene *gqlScene `json:"findScene"`
	}
	if err := a.gql(ctx, findSceneByIDQuery, map[string]any{"id": id}, &resp); err != nil {
		return nil, err
	}
	if resp.FindScene == nil {
		return nil, ports.ErrNotFound
	}
	return toExternalItem(resp.FindScene, domain.ContentTypeAdult), nil
}

// FetchEntryContent pages through all scenes for a studio. StashDB scenes are
// flat — groups is always nil; items contains the page of scenes; total is the
// scene count across all pages.
func (a *Adapter) FetchEntryContent(ctx context.Context, _ domain.ContentType, externalID string, page, perPage int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
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
func (a *Adapter) FetchGroupContent(_ context.Context, _ domain.ContentType, _ string, _, _ int) ([]*domain.ExternalItem, int, error) {
	return nil, 0, ports.ErrNotSupported
}

// FetchEntryPeople returns ErrNotSupported — StashDB does not model studio membership.
func (a *Adapter) FetchEntryPeople(_ context.Context, _ string) ([]*domain.ExternalPerson, error) {
	return nil, ports.ErrNotSupported
}

// FindGroupImages returns ErrNotSupported — StashDB scenes are flat; there is no group-level image concept.
func (a *Adapter) FindGroupImages(_ context.Context, _ domain.ContentType, _, _ string) (*domain.ExternalItem, error) {
	return nil, ports.ErrNotSupported
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
		images := make([]domain.ExternalImage, 0, len(s.Images))
		for _, img := range s.Images {
			if img.URL != "" {
				images = append(images, domain.ExternalImage{Type: domain.ImageTypeThumbnail, URL: img.URL})
			}
		}
		e.Images = images
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
