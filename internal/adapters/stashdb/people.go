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
	Gender          string       `json:"gender"` // StashDB enum; normalized on mapping
	Ethnicity       string       `json:"ethnicity"`
	Nationality     string       `json:"country"` // StashDB uses "country"; maps to "nationality"
	BreastType      string       `json:"breast_type"`
	CareerStartYear int          `json:"career_start_year"`
	CareerEndYear   int          `json:"career_end_year"`
	Disambiguation  string       `json:"disambiguation"`
	Tattoos         []gqlBodyMod `json:"tattoos"`
	Piercings       []gqlBodyMod `json:"piercings"`

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
      gender
      ethnicity
      country
      breast_type
      career_start_year
      career_end_year
      disambiguation
      tattoos { location description }
      piercings { location description }
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
	if p.Height > 0 {
		m["height"] = fmt.Sprintf("%d cm", p.Height)
	}
	if p.HairColor != "" {
		m["hair_color"] = strings.ToLower(p.HairColor)
	}
	if p.EyeColor != "" {
		m["eye_color"] = strings.ToLower(p.EyeColor)
	}
	if p.CareerStartYear > 0 {
		m["career_start"] = fmt.Sprintf("%d", p.CareerStartYear)
	}
	if s := bodyModString(p.Tattoos); s != "" {
		m["tattoos"] = s
	}
	if s := bodyModString(p.Piercings); s != "" {
		m["piercings"] = s
	}
	applyMeasurements(m, p.Measurements)
	applyExtendedMetadata(m, p)

	if len(m) == 0 {
		return nil
	}
	return m
}

func applyExtendedMetadata(m map[string]any, p *gqlPerformer) {
	if p.Gender != "" {
		m["gender"] = normalizeGender(p.Gender)
	}
	if p.Ethnicity != "" {
		m["ethnicity"] = p.Ethnicity
	}
	if p.Nationality != "" {
		m["nationality"] = p.Nationality
	}
	if p.CareerEndYear > 0 {
		m["career_end"] = fmt.Sprintf("%d", p.CareerEndYear)
	}
	switch strings.ToUpper(p.BreastType) {
	case "FAKE":
		m["breast_type"] = "Fake"
	case "NATURAL":
		m["breast_type"] = "Natural"
	}
	if p.Disambiguation != "" {
		m["disambiguation"] = p.Disambiguation
	}
}

// normalizeGender maps StashDB gender enum values to the canonical lowercase
// string values defined in docs/person-metadata-keys.md.
func normalizeGender(g string) string {
	switch strings.ToUpper(g) {
	case "MALE":
		return "male"
	case "FEMALE":
		return "female"
	case "TRANSGENDER_MALE":
		return "transgender_male"
	case "TRANSGENDER_FEMALE":
		return "transgender_female"
	case "INTERSEX":
		return "intersex"
	case "NON_BINARY":
		return "non_binary"
	default:
		return "unknown"
	}
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
