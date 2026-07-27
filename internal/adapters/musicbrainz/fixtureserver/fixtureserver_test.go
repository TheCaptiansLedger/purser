package fixtureserver_test

import (
	"context"
	"purser/internal/adapters/musicbrainz"
	"purser/internal/adapters/musicbrainz/fixtureserver"
	"testing"
)

func newFixtureClient(t *testing.T) *musicbrainz.Client {
	t.Helper()
	c, err := musicbrainz.New(musicbrainz.DefaultConfig(), musicbrainz.WithBaseTransport(fixtureserver.Transport()))
	if err != nil {
		t.Fatalf("musicbrainz.New returned error: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestTransport_LookupArtist_ResolvesBothFixtureArtists(t *testing.T) {
	c := newFixtureClient(t)

	a, err := c.LookupArtist(context.Background(), fixtureserver.ArtistMBID)
	if err != nil {
		t.Fatalf("LookupArtist(%s) returned error: %v", fixtureserver.ArtistMBID, err)
	}
	if a.Name != fixtureserver.ArtistName {
		t.Errorf("Name = %q, want %q", a.Name, fixtureserver.ArtistName)
	}

	a2, err := c.LookupArtist(context.Background(), fixtureserver.ArtistMBID2)
	if err != nil {
		t.Fatalf("LookupArtist(%s) returned error: %v", fixtureserver.ArtistMBID2, err)
	}
	if a2.Name != fixtureserver.ArtistName2 {
		t.Errorf("Name = %q, want %q", a2.Name, fixtureserver.ArtistName2)
	}
}

func TestTransport_LookupRelease_ResolvesBothFixtureReleases(t *testing.T) {
	c := newFixtureClient(t)

	r, err := c.LookupRelease(context.Background(), fixtureserver.ReleaseMBID)
	if err != nil {
		t.Fatalf("LookupRelease(%s) returned error: %v", fixtureserver.ReleaseMBID, err)
	}
	if len(r.Media) != 1 || len(r.Media[0].Tracks) != 2 {
		t.Fatalf("ReleaseMBID = %+v, want one medium with two tracks", r)
	}

	r2, err := c.LookupRelease(context.Background(), fixtureserver.ReleaseMBID2)
	if err != nil {
		t.Fatalf("LookupRelease(%s) returned error: %v", fixtureserver.ReleaseMBID2, err)
	}
	if len(r2.Media) != 1 || len(r2.Media[0].Tracks) != 1 {
		t.Fatalf("ReleaseMBID2 = %+v, want one medium with one track", r2)
	}
}

func TestTransport_SearchRoutes_AlwaysReturnEmpty(t *testing.T) {
	c := newFixtureClient(t)

	artists, err := c.SearchArtists(context.Background(), "anything")
	if err != nil {
		t.Fatalf("SearchArtists returned error: %v", err)
	}
	if len(artists) != 0 {
		t.Errorf("SearchArtists = %+v, want empty", artists)
	}

	rgs, err := c.SearchReleaseGroups(context.Background(), "anything", "anything")
	if err != nil {
		t.Fatalf("SearchReleaseGroups returned error: %v", err)
	}
	if len(rgs) != 0 {
		t.Errorf("SearchReleaseGroups = %+v, want empty", rgs)
	}

	releases, err := c.SearchReleaseByBarcode(context.Background(), "anything")
	if err != nil {
		t.Fatalf("SearchReleaseByBarcode returned error: %v", err)
	}
	if len(releases) != 0 {
		t.Errorf("SearchReleaseByBarcode = %+v, want empty", releases)
	}
}
