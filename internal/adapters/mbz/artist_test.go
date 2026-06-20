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

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
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

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
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

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
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

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	_, err := a.FetchEntryPeople(context.Background(), "does-not-exist")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Errorf("expected ports.ErrNotFound, got: %v", err)
	}
}

func TestFetchEntryPeople_EmptyRelations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id": "solo-mbid", "relations": []}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := mbz.New(config.MetadataSourceConfig{URL: srv.URL})
	members, err := a.FetchEntryPeople(context.Background(), "solo-mbid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 0 {
		t.Errorf("len(members) = %d, want 0 for solo artist", len(members))
	}
}
