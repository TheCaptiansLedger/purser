package stashdb

import (
	"context"
	"fmt"
	"purser/internal/domain"
	"strings"
)

// ── GraphQL response types ────────────────────────────────────────────────────

type gqlBodyMod struct {
	Location    string `json:"location"`
	Description string `json:"description"`
}

type gqlPerformer struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Aliases []string   `json:"aliases"`
	Images  []gqlImage `json:"images"`

	Birthdate *struct {
		Date string `json:"date"`
	} `json:"birthdate"`

	Height          int          `json:"height"` // centimetres
	HairColor       string       `json:"hair_color"`
	EyeColor        string       `json:"eye_color"`
	Tattoos         []gqlBodyMod `json:"tattoos"`
	Piercings       []gqlBodyMod `json:"piercings"`
	CareerStartYear int          `json:"career_start_year"`

	Measurements *struct {
		CupSize  string `json:"cup_size"`
		BandSize int    `json:"band_size"`
		Waist    int    `json:"waist"`
		Hip      int    `json:"hip"`
	} `json:"measurements"`
}

// ── Queries ───────────────────────────────────────────────────────────────────

const searchPeopleQuery = `
query SearchPerformers($name: String!, $limit: Int!) {
  queryPerformers(input: {
    name: $name
    per_page: $limit
  }) {
    performers {
      id
      name
      aliases
      images { url }
      birthdate { date }
      height
      hair_color
      eye_color
      tattoos { location description }
      piercings { location description }
      career_start_year
      measurements { cup_size band_size waist hip }
    }
  }
}`

// ── MetadataSource ────────────────────────────────────────────────────────────

// SearchPeople queries StashDB for performers matching the given search string.
func (a *Adapter) SearchPeople(ctx context.Context, query string, limit int) ([]*domain.ExternalPerson, error) {
	var resp struct {
		QueryPerformers struct {
			Performers []gqlPerformer `json:"performers"`
		} `json:"queryPerformers"`
	}
	if err := a.gql(ctx, searchPeopleQuery, map[string]any{"name": query, "limit": limit}, &resp); err != nil {
		return nil, err
	}

	perfs := resp.QueryPerformers.Performers
	out := make([]*domain.ExternalPerson, len(perfs))
	for i := range perfs {
		out[i] = toExternalPerson(&perfs[i])
	}
	return out, nil
}

// ── Mapping ───────────────────────────────────────────────────────────────────

func toExternalPerson(p *gqlPerformer) *domain.ExternalPerson {
	e := &domain.ExternalPerson{
		Source:     domain.SourceStashDB,
		ExternalID: p.ID,
		Name:       p.Name,
		Aliases:    p.Aliases,
		Role:       domain.RolePerformer,
		Metadata:   performerMetadata(p),
	}
	if len(p.Images) > 0 {
		e.ImageURL = p.Images[0].URL
	}
	return e
}

func performerMetadata(p *gqlPerformer) map[string]any {
	m := make(map[string]any)

	if p.Birthdate != nil && p.Birthdate.Date != "" {
		m["birthdate"] = p.Birthdate.Date
	}
	if p.HairColor != "" {
		m["hair_color"] = strings.ToLower(p.HairColor)
	}
	if p.EyeColor != "" {
		m["eye_color"] = strings.ToLower(p.EyeColor)
	}
	if p.Height > 0 {
		m["height"] = fmt.Sprintf("%d cm", p.Height)
	}
	if s := bodyModString(p.Tattoos); s != "" {
		m["tattoos"] = s
	}
	if s := bodyModString(p.Piercings); s != "" {
		m["piercings"] = s
	}
	if p.CareerStartYear > 0 {
		m["career_start"] = fmt.Sprintf("%d", p.CareerStartYear)
	}
	applyMeasurements(m, p.Measurements)

	if len(m) == 0 {
		return nil
	}
	return m
}

func applyMeasurements(m map[string]any, ms *struct {
	CupSize  string `json:"cup_size"`
	BandSize int    `json:"band_size"`
	Waist    int    `json:"waist"`
	Hip      int    `json:"hip"`
},
) {
	if ms == nil {
		return
	}
	if ms.CupSize != "" {
		m["cup_size"] = ms.CupSize
	}
	var parts []string
	if ms.BandSize > 0 {
		parts = append(parts, fmt.Sprintf("%d", ms.BandSize))
	}
	if ms.Waist > 0 {
		parts = append(parts, fmt.Sprintf("%d", ms.Waist))
	}
	if ms.Hip > 0 {
		parts = append(parts, fmt.Sprintf("%d", ms.Hip))
	}
	if len(parts) > 0 {
		m["measurements"] = strings.Join(parts, "-")
	}
}

func bodyModString(mods []gqlBodyMod) string {
	if len(mods) == 0 {
		return ""
	}
	parts := make([]string, 0, len(mods))
	for _, mod := range mods {
		if mod.Description != "" {
			parts = append(parts, mod.Location+": "+mod.Description)
		} else if mod.Location != "" {
			parts = append(parts, mod.Location)
		}
	}
	return strings.Join(parts, "; ")
}
