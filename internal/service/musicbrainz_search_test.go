package service_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// fakeMusicBrainzClient is a minimal ports.MusicBrainzClient double —
// MusicBrainzSearch only ever calls SearchReleaseGroups/
// ListReleasesForReleaseGroup, so every other method is an unused stub,
// per docs/adr/0003-go-testing-standards.md's "services fake the ports
// they consume" rule.
type fakeMusicBrainzClient struct {
	releaseGroups          []ports.ReleaseGroup
	releaseGroupsErr       error
	releases               []ports.Release
	releasesErr            error
	artists                []ports.Artist
	artistsErr             error
	artistReleaseGroups    []ports.ReleaseGroup
	artistReleaseGroupsErr error
	artist                 *ports.Artist
	artistErr              error

	gotArtistName, gotAlbumName string
	gotReleaseGroupMBID         string
	gotQuery                    string
	gotArtistMBID               string
	gotMBID                     string
}

var _ ports.MusicBrainzClient = (*fakeMusicBrainzClient)(nil)

func (f *fakeMusicBrainzClient) LookupArtist(_ context.Context, mbid string) (*ports.Artist, error) {
	f.gotMBID = mbid
	if f.artistErr != nil {
		return nil, f.artistErr
	}
	return f.artist, nil
}

func (f *fakeMusicBrainzClient) SearchArtists(_ context.Context, query string) ([]ports.Artist, error) {
	f.gotQuery = query
	if f.artistsErr != nil {
		return nil, f.artistsErr
	}
	return f.artists, nil
}

func (f *fakeMusicBrainzClient) LookupReleaseGroup(context.Context, string) (*ports.ReleaseGroup, error) {
	return nil, ports.ErrNotFound
}

func (f *fakeMusicBrainzClient) ListReleaseGroupsForArtist(_ context.Context, artistMBID string) ([]ports.ReleaseGroup, error) {
	f.gotArtistMBID = artistMBID
	if f.artistReleaseGroupsErr != nil {
		return nil, f.artistReleaseGroupsErr
	}
	return f.artistReleaseGroups, nil
}

func (f *fakeMusicBrainzClient) SearchReleaseGroups(_ context.Context, artistName, albumName string) ([]ports.ReleaseGroup, error) {
	f.gotArtistName, f.gotAlbumName = artistName, albumName
	if f.releaseGroupsErr != nil {
		return nil, f.releaseGroupsErr
	}
	return f.releaseGroups, nil
}

func (f *fakeMusicBrainzClient) LookupRelease(context.Context, string) (*ports.Release, error) {
	return nil, ports.ErrNotFound
}

func (f *fakeMusicBrainzClient) ListReleasesForReleaseGroup(_ context.Context, rgMBID string) ([]ports.Release, error) {
	f.gotReleaseGroupMBID = rgMBID
	if f.releasesErr != nil {
		return nil, f.releasesErr
	}
	return f.releases, nil
}

func (f *fakeMusicBrainzClient) SearchReleaseByBarcode(context.Context, string) ([]ports.Release, error) {
	return nil, nil
}

func (f *fakeMusicBrainzClient) LookupRecordingByISRC(context.Context, string) ([]ports.Recording, error) {
	return nil, nil
}

func TestMusicBrainzSearch_SearchReleaseGroups_PassesQueryThrough(t *testing.T) {
	want := []ports.ReleaseGroup{{ID: "rg-1", Title: "Hi Infidelity"}}
	mb := &fakeMusicBrainzClient{releaseGroups: want}
	s := service.NewMusicBrainzSearch(mb)

	got, err := s.SearchReleaseGroups(context.Background(), "REO Speedwagon", "Hi Infidelity")
	if err != nil {
		t.Fatalf("SearchReleaseGroups returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "rg-1" {
		t.Errorf("SearchReleaseGroups = %+v, want %+v", got, want)
	}
	if mb.gotArtistName != "REO Speedwagon" || mb.gotAlbumName != "Hi Infidelity" {
		t.Errorf("SearchReleaseGroups called with artist=%q album=%q, want REO Speedwagon/Hi Infidelity", mb.gotArtistName, mb.gotAlbumName)
	}
}

func TestMusicBrainzSearch_SearchReleaseGroups_PropagatesError(t *testing.T) {
	wantErr := errors.New("musicbrainz unavailable")
	mb := &fakeMusicBrainzClient{releaseGroupsErr: wantErr}
	s := service.NewMusicBrainzSearch(mb)

	if _, err := s.SearchReleaseGroups(context.Background(), "x", "y"); !errors.Is(err, wantErr) {
		t.Fatalf("SearchReleaseGroups returned %v, want %v", err, wantErr)
	}
}

func TestMusicBrainzSearch_ListReleasesForReleaseGroup_PassesMBIDThrough(t *testing.T) {
	want := []ports.Release{{ID: "release-1", Title: "Hi Infidelity"}}
	mb := &fakeMusicBrainzClient{releases: want}
	s := service.NewMusicBrainzSearch(mb)

	got, err := s.ListReleasesForReleaseGroup(context.Background(), "rg-1")
	if err != nil {
		t.Fatalf("ListReleasesForReleaseGroup returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "release-1" {
		t.Errorf("ListReleasesForReleaseGroup = %+v, want %+v", got, want)
	}
	if mb.gotReleaseGroupMBID != "rg-1" {
		t.Errorf("ListReleasesForReleaseGroup called with %q, want rg-1", mb.gotReleaseGroupMBID)
	}
}

func TestMusicBrainzSearch_ListReleasesForReleaseGroup_PropagatesError(t *testing.T) {
	wantErr := errors.New("musicbrainz unavailable")
	mb := &fakeMusicBrainzClient{releasesErr: wantErr}
	s := service.NewMusicBrainzSearch(mb)

	if _, err := s.ListReleasesForReleaseGroup(context.Background(), "rg-1"); !errors.Is(err, wantErr) {
		t.Fatalf("ListReleasesForReleaseGroup returned %v, want %v", err, wantErr)
	}
}

func TestMusicBrainzSearch_SearchArtists_PassesQueryThrough(t *testing.T) {
	want := []ports.Artist{{ID: "artist-1", Name: "REO Speedwagon"}}
	mb := &fakeMusicBrainzClient{artists: want}
	s := service.NewMusicBrainzSearch(mb)

	got, err := s.SearchArtists(context.Background(), "REO Speedwagon")
	if err != nil {
		t.Fatalf("SearchArtists returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "artist-1" {
		t.Errorf("SearchArtists = %+v, want %+v", got, want)
	}
	if mb.gotQuery != "REO Speedwagon" {
		t.Errorf("SearchArtists called with query=%q, want REO Speedwagon", mb.gotQuery)
	}
}

func TestMusicBrainzSearch_SearchArtists_EmptyResultIsNotAnError(t *testing.T) {
	mb := &fakeMusicBrainzClient{artists: nil}
	s := service.NewMusicBrainzSearch(mb)

	got, err := s.SearchArtists(context.Background(), "no such artist")
	if err != nil {
		t.Fatalf("SearchArtists returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("SearchArtists = %+v, want empty", got)
	}
}

func TestMusicBrainzSearch_SearchArtists_PropagatesError(t *testing.T) {
	wantErr := errors.New("musicbrainz unavailable")
	mb := &fakeMusicBrainzClient{artistsErr: wantErr}
	s := service.NewMusicBrainzSearch(mb)

	if _, err := s.SearchArtists(context.Background(), "x"); !errors.Is(err, wantErr) {
		t.Fatalf("SearchArtists returned %v, want %v", err, wantErr)
	}
}

func TestMusicBrainzSearch_ListReleaseGroupsForArtist_PassesMBIDThrough(t *testing.T) {
	want := []ports.ReleaseGroup{{ID: "rg-1", Title: "Hi Infidelity"}}
	mb := &fakeMusicBrainzClient{artistReleaseGroups: want}
	s := service.NewMusicBrainzSearch(mb)

	got, err := s.ListReleaseGroupsForArtist(context.Background(), "artist-1")
	if err != nil {
		t.Fatalf("ListReleaseGroupsForArtist returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "rg-1" {
		t.Errorf("ListReleaseGroupsForArtist = %+v, want %+v", got, want)
	}
	if mb.gotArtistMBID != "artist-1" {
		t.Errorf("ListReleaseGroupsForArtist called with %q, want artist-1", mb.gotArtistMBID)
	}
}

func TestMusicBrainzSearch_ListReleaseGroupsForArtist_EmptyResultIsNotAnError(t *testing.T) {
	mb := &fakeMusicBrainzClient{artistReleaseGroups: nil}
	s := service.NewMusicBrainzSearch(mb)

	got, err := s.ListReleaseGroupsForArtist(context.Background(), "artist-1")
	if err != nil {
		t.Fatalf("ListReleaseGroupsForArtist returned error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListReleaseGroupsForArtist = %+v, want empty", got)
	}
}

func TestMusicBrainzSearch_ListReleaseGroupsForArtist_PropagatesError(t *testing.T) {
	wantErr := errors.New("musicbrainz unavailable")
	mb := &fakeMusicBrainzClient{artistReleaseGroupsErr: wantErr}
	s := service.NewMusicBrainzSearch(mb)

	if _, err := s.ListReleaseGroupsForArtist(context.Background(), "artist-1"); !errors.Is(err, wantErr) {
		t.Fatalf("ListReleaseGroupsForArtist returned %v, want %v", err, wantErr)
	}
}

func TestMusicBrainzSearch_GetArtist_PassesMBIDThrough(t *testing.T) {
	want := &ports.Artist{ID: "artist-1", Name: "REO Speedwagon"}
	mb := &fakeMusicBrainzClient{artist: want}
	s := service.NewMusicBrainzSearch(mb)

	got, err := s.GetArtist(context.Background(), "artist-1")
	if err != nil {
		t.Fatalf("GetArtist returned error: %v", err)
	}
	if got.ID != "artist-1" {
		t.Errorf("GetArtist = %+v, want %+v", got, want)
	}
	if mb.gotMBID != "artist-1" {
		t.Errorf("GetArtist called with %q, want artist-1", mb.gotMBID)
	}
}

func TestMusicBrainzSearch_GetArtist_PropagatesNotFound(t *testing.T) {
	mb := &fakeMusicBrainzClient{artistErr: ports.ErrNotFound}
	s := service.NewMusicBrainzSearch(mb)

	if _, err := s.GetArtist(context.Background(), "unknown-mbid"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetArtist returned %v, want ports.ErrNotFound", err)
	}
}
