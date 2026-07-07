package mbz_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/mbz"
	"purser/internal/config"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

func TestFindByExternalID_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"abc-123","name":"Nirvana","disambiguation":"90s US grunge band"}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	item, err := a.FindByExternalID(context.Background(), domain.ContentTypeMusic, "abc-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.ExternalID != "abc-123" {
		t.Errorf("ExternalID = %q, want abc-123", item.ExternalID)
	}
	if item.Title != "Nirvana" {
		t.Errorf("Title = %q, want Nirvana", item.Title)
	}
	if item.Overview != "90s US grunge band" {
		t.Errorf("Overview = %q, want 90s US grunge band", item.Overview)
	}
}

func TestFindByExternalID_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	_, err := a.FindByExternalID(context.Background(), domain.ContentTypeMusic, "does-not-exist")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Errorf("expected ports.ErrNotFound, got: %v", err)
	}
}

func TestFetchEntryPeople_Success(t *testing.T) {
	const fixture = `{
		"id": "group-mbid",
		"relations": [
			{"type": "member of band", "direction": "backward", "artist": {"id": "m1", "name": "Mick Fleetwood"}},
			{"type": "member of band", "direction": "backward", "artist": {"id": "m2", "name": "Stevie Nicks"}},
			{"type": "member of band", "direction": "forward",  "artist": {"id": "m3", "name": "Not A Member"}},
			{"type": "founder",        "direction": "backward", "artist": {"id": "m4", "name": "Also Not A Member"}}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fixture)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	members, err := a.FetchEntryPeople(context.Background(), "group-mbid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("len(members) = %d, want 2 (only backward member-of-band relations)", len(members))
	}
	if members[0].ExternalID != "m1" || members[0].Name != "Mick Fleetwood" {
		t.Errorf("members[0] = {%q, %q}, want {m1, Mick Fleetwood}", members[0].ExternalID, members[0].Name)
	}
	if members[1].ExternalID != "m2" || members[1].Name != "Stevie Nicks" {
		t.Errorf("members[1] = {%q, %q}, want {m2, Stevie Nicks}", members[1].ExternalID, members[1].Name)
	}
	if members[0].Role != domain.RoleArtist {
		t.Errorf("members[0].Role = %q, want %q", members[0].Role, domain.RoleArtist)
	}
}

func TestFetchEntryPeople_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	_, err := a.FetchEntryPeople(context.Background(), "does-not-exist")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Errorf("expected ports.ErrNotFound, got: %v", err)
	}
}

// ── FetchEntryMetadata ────────────────────────────────────────────────────────

const enrichFixtureGroup = `{
	"id": "reo-mbid",
	"name": "REO Speedwagon",
	"type": "Group",
	"life-span": {"begin": "1967", "end": ""},
	"begin-area": {"name": "Champaign"},
	"isnis": ["0000000121210798"],
	"aliases": [
		{"name": "REO Speedwagon",  "type": "Artist name",    "locale": ""},
		{"name": "Reo Speedwagon",  "type": "Search hint",    "locale": "en"},
		{"name": "R E O Speedwagon","type": null,              "locale": null},
		{"name": "Reo Speedvagon",  "type": "Transliteration","locale": "ja"},
		{"name": "REO",             "type": "Artist name",    "locale": "de"}
	],
	"relations": [
		{"type": "official homepage", "direction": "forward",  "url": {"resource": "https://reospeedwagon.com"}},
		{"type": "last.fm",           "direction": "forward",  "url": {"resource": "https://www.last.fm/music/REO+Speedwagon"}},
		{"type": "wikipedia",         "direction": "forward",  "url": {"resource": "https://en.wikipedia.org/wiki/REO_Speedwagon"}},
		{"type": "member of band",    "direction": "backward", "url": {"resource": ""}}
	]
}`

const enrichFixturePerson = `{
	"id": "stevie-mbid",
	"name": "Stevie Nicks",
	"type": "Person",
	"life-span": {"begin": "1948-05-26", "end": ""},
	"begin-area": {"name": "Phoenix"},
	"isnis": [],
	"aliases": [],
	"relations": []
}`

func TestMBZArtist_ParsesAliases_FiltersTypes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(enrichFixtureGroup)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	meta, err := a.FetchEntryMetadata(context.Background(), domain.ContentTypeMusic, "reo-mbid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	aliases, ok := meta["aliases"].([]string)
	if !ok {
		t.Fatal("meta[aliases] is not []string")
	}
	// "Reo Speedvagon" (Transliteration) and "REO" (locale "de") must be excluded.
	// null-type aliases with null locale must be included.
	if len(aliases) != 3 {
		t.Errorf("len(aliases) = %d, want 3 (artist-name, search-hint+en, null-type+null-locale)", len(aliases))
	}
	for _, alias := range aliases {
		if alias == "Reo Speedvagon" || alias == "REO" {
			t.Errorf("alias %q should have been filtered out", alias)
		}
	}
}

func TestMBZArtist_ParsesLifeSpan_Group(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(enrichFixtureGroup)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	meta, err := a.FetchEntryMetadata(context.Background(), domain.ContentTypeMusic, "reo-mbid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta["founded_date"] != "1967" {
		t.Errorf("founded_date = %q, want 1967", meta["founded_date"])
	}
	if meta["founded_location"] != "Champaign" {
		t.Errorf("founded_location = %q, want Champaign", meta["founded_location"])
	}
	if meta["artist_type"] != "group" {
		t.Errorf("artist_type = %q, want group", meta["artist_type"])
	}
	if _, ok := meta["born_date"]; ok {
		t.Error("born_date must not be set for a group artist")
	}
	if _, ok := meta["born_location"]; ok {
		t.Error("born_location must not be set for a group artist")
	}
}

func TestMBZArtist_ParsesLifeSpan_Person(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(enrichFixturePerson)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	meta, err := a.FetchEntryMetadata(context.Background(), domain.ContentTypeMusic, "stevie-mbid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta["born_date"] != "1948-05-26" {
		t.Errorf("born_date = %q, want 1948-05-26", meta["born_date"])
	}
	if meta["born_location"] != "Phoenix" {
		t.Errorf("born_location = %q, want Phoenix", meta["born_location"])
	}
	if meta["artist_type"] != "person" {
		t.Errorf("artist_type = %q, want person", meta["artist_type"])
	}
	if _, ok := meta["founded_date"]; ok {
		t.Error("founded_date must not be set for a person artist")
	}
}

func TestMBZArtist_ParsesURLRelations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(enrichFixtureGroup)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	meta, err := a.FetchEntryMetadata(context.Background(), domain.ContentTypeMusic, "reo-mbid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta["official_url"] != "https://reospeedwagon.com" {
		t.Errorf("official_url = %q, want https://reospeedwagon.com", meta["official_url"])
	}
	if meta["lastfm_url"] != "https://www.last.fm/music/REO+Speedwagon" {
		t.Errorf("lastfm_url = %q, want https://www.last.fm/music/REO+Speedwagon", meta["lastfm_url"])
	}
	if meta["wikipedia_url"] != "https://en.wikipedia.org/wiki/REO_Speedwagon" {
		t.Errorf("wikipedia_url = %q, want https://en.wikipedia.org/wiki/REO_Speedwagon", meta["wikipedia_url"])
	}
}

func TestMBZArtist_ParsesISNI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(enrichFixtureGroup)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	meta, err := a.FetchEntryMetadata(context.Background(), domain.ContentTypeMusic, "reo-mbid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if meta["isni"] != "0000000121210798" {
		t.Errorf("isni = %q, want 0000000121210798", meta["isni"])
	}
}

func TestFetchEntryPeople_EmptyRelations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": "solo-mbid", "relations": []}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL}, nil)
	members, err := a.FetchEntryPeople(context.Background(), "solo-mbid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 0 {
		t.Errorf("len(members) = %d, want 0 for solo artist", len(members))
	}
}
