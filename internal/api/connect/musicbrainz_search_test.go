package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"

	musicv1 "purser/gen/go/purser/music/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeMusicBrainzSearchService struct {
	gotArtistName, gotAlbumName string
	returnReleaseGroups         []ports.ReleaseGroup
	releaseGroupsErr            error

	gotReleaseGroupMBID string
	returnReleases      []ports.Release
	releasesErr         error

	gotQuery      string
	returnArtists []ports.Artist
	artistsErr    error

	gotArtistMBID             string
	returnArtistReleaseGroups []ports.ReleaseGroup
	artistReleaseGroupsErr    error

	gotMBID      string
	returnArtist *ports.Artist
	artistErr    error

	gotReleaseMBID string
	returnRelease  *ports.Release
	releaseErr     error
}

func (f *fakeMusicBrainzSearchService) SearchReleaseGroups(_ context.Context, artistName, albumName string) ([]ports.ReleaseGroup, error) {
	f.gotArtistName, f.gotAlbumName = artistName, albumName
	if f.releaseGroupsErr != nil {
		return nil, f.releaseGroupsErr
	}
	return f.returnReleaseGroups, nil
}

func (f *fakeMusicBrainzSearchService) ListReleasesForReleaseGroup(_ context.Context, releaseGroupMBID string) ([]ports.Release, error) {
	f.gotReleaseGroupMBID = releaseGroupMBID
	if f.releasesErr != nil {
		return nil, f.releasesErr
	}
	return f.returnReleases, nil
}

func (f *fakeMusicBrainzSearchService) SearchArtists(_ context.Context, query string) ([]ports.Artist, error) {
	f.gotQuery = query
	if f.artistsErr != nil {
		return nil, f.artistsErr
	}
	return f.returnArtists, nil
}

func (f *fakeMusicBrainzSearchService) ListReleaseGroupsForArtist(_ context.Context, artistMBID string) ([]ports.ReleaseGroup, error) {
	f.gotArtistMBID = artistMBID
	if f.artistReleaseGroupsErr != nil {
		return nil, f.artistReleaseGroupsErr
	}
	return f.returnArtistReleaseGroups, nil
}

func (f *fakeMusicBrainzSearchService) GetArtist(_ context.Context, mbid string) (*ports.Artist, error) {
	f.gotMBID = mbid
	if f.artistErr != nil {
		return nil, f.artistErr
	}
	return f.returnArtist, nil
}

func (f *fakeMusicBrainzSearchService) GetRelease(_ context.Context, mbid string) (*ports.Release, error) {
	f.gotReleaseMBID = mbid
	if f.releaseErr != nil {
		return nil, f.releaseErr
	}
	return f.returnRelease, nil
}

func TestMusicBrainzSearchHandler_SearchReleaseGroups(t *testing.T) {
	t.Run("valid request returns release groups and passes the query through", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{returnReleaseGroups: []ports.ReleaseGroup{
			{ID: "rg-1", Title: "Hi Infidelity", PrimaryType: "Album", FirstReleaseDate: "1980-11-21"},
		}}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		res, err := h.SearchReleaseGroups(context.Background(), connect.NewRequest(&musicv1.SearchMusicBrainzReleaseGroupsRequest{
			ArtistName: "REO Speedwagon",
			AlbumName:  "Hi Infidelity",
		}))
		if err != nil {
			t.Fatalf("SearchReleaseGroups returned error: %v", err)
		}
		if len(res.Msg.GetReleaseGroups()) != 1 {
			t.Fatalf("SearchReleaseGroups returned %d release groups, want 1", len(res.Msg.GetReleaseGroups()))
		}
		got := res.Msg.GetReleaseGroups()[0]
		if got.GetMbid() != "rg-1" || got.GetTitle() != "Hi Infidelity" || got.GetPrimaryType() != "Album" || got.GetFirstReleaseDate() != "1980-11-21" {
			t.Errorf("SearchReleaseGroups returned %+v, want a match for the fake's release group", got)
		}
		if svc.gotArtistName != "REO Speedwagon" || svc.gotAlbumName != "Hi Infidelity" {
			t.Errorf("SearchReleaseGroups passed artist=%q album=%q, want REO Speedwagon/Hi Infidelity", svc.gotArtistName, svc.gotAlbumName)
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{releaseGroupsErr: errors.New("boom")}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		_, err := h.SearchReleaseGroups(context.Background(), connect.NewRequest(&musicv1.SearchMusicBrainzReleaseGroupsRequest{ArtistName: "x", AlbumName: "y"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("SearchReleaseGroups with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}

func TestMusicBrainzSearchHandler_ListReleasesForReleaseGroup(t *testing.T) {
	t.Run("valid request returns releases summarized correctly", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{returnReleases: []ports.Release{
			{
				ID: "release-1", Title: "Hi Infidelity", Country: "US", Date: "2000-11-21", Barcode: "075992599720",
				ArtistCredit: []ports.ArtistCredit{{Name: "REO Speedwagon"}},
				LabelInfo:    []ports.LabelInfo{{Label: ports.Label{Name: "Epic"}, CatalogNumber: "E2 85369"}},
				Media: []ports.Medium{{
					// TrackCount, not len(Tracks) — regression coverage for
					// the real bug found live against the API:
					// ListReleasesForReleaseGroup's inc= never requests
					// "recordings", so Tracks is always empty; only the
					// medium's own TrackCount scalar is ever populated.
					Format:     "CD",
					TrackCount: 2,
					Tracks:     nil,
				}},
			},
		}}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		res, err := h.ListReleasesForReleaseGroup(context.Background(), connect.NewRequest(&musicv1.ListMusicBrainzReleasesRequest{ReleaseGroupMbid: "rg-1"}))
		if err != nil {
			t.Fatalf("ListReleasesForReleaseGroup returned error: %v", err)
		}
		if len(res.Msg.GetReleases()) != 1 {
			t.Fatalf("ListReleasesForReleaseGroup returned %d releases, want 1", len(res.Msg.GetReleases()))
		}
		got := res.Msg.GetReleases()[0]
		if got.GetMbid() != "release-1" || got.GetCountry() != "US" || got.GetBarcode() != "075992599720" {
			t.Errorf("release = %+v, want mbid=release-1 country=US barcode=075992599720", got)
		}
		if got.GetFormat() != "CD" || got.GetMediumCount() != 1 || got.GetTrackCount() != 2 {
			t.Errorf("release medium summary = format=%q medium_count=%d track_count=%d, want CD/1/2", got.GetFormat(), got.GetMediumCount(), got.GetTrackCount())
		}
		if len(got.GetArtistCreditNames()) != 1 || got.GetArtistCreditNames()[0] != "REO Speedwagon" {
			t.Errorf("ArtistCreditNames = %v, want [REO Speedwagon]", got.GetArtistCreditNames())
		}
		if got.GetLabel() != "Epic" {
			t.Errorf("Label = %q, want Epic", got.GetLabel())
		}
		if got.GetCatalogNumber() != "E2 85369" {
			t.Errorf("CatalogNumber = %q, want E2 85369", got.GetCatalogNumber())
		}
		if svc.gotReleaseGroupMBID != "rg-1" {
			t.Errorf("ListReleasesForReleaseGroup passed release_group_mbid=%q, want rg-1", svc.gotReleaseGroupMBID)
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{releasesErr: errors.New("boom")}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		_, err := h.ListReleasesForReleaseGroup(context.Background(), connect.NewRequest(&musicv1.ListMusicBrainzReleasesRequest{ReleaseGroupMbid: "rg-1"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListReleasesForReleaseGroup with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}

func TestMusicBrainzSearchHandler_SearchArtists(t *testing.T) {
	t.Run("valid request returns artists and passes the query through", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{returnArtists: []ports.Artist{
			{
				ID: "artist-1", Name: "REO Speedwagon", SortName: "REO Speedwagon", Type: "Group", Country: "US",
				LifeSpan: ports.LifeSpan{Begin: "1967", End: ""},
				Aliases:  []ports.Alias{{Name: "R.E.O. Speedwagon"}},
			},
		}}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		res, err := h.SearchArtists(context.Background(), connect.NewRequest(&musicv1.SearchMusicBrainzArtistsRequest{Query: "REO Speedwagon"}))
		if err != nil {
			t.Fatalf("SearchArtists returned error: %v", err)
		}
		if len(res.Msg.GetArtists()) != 1 {
			t.Fatalf("SearchArtists returned %d artists, want 1", len(res.Msg.GetArtists()))
		}
		got := res.Msg.GetArtists()[0]
		if got.GetMbid() != "artist-1" || got.GetName() != "REO Speedwagon" || got.GetType() != "Group" || got.GetCountry() != "US" {
			t.Errorf("SearchArtists returned %+v, want a match for the fake's artist", got)
		}
		if got.GetLifeSpanBegin() != "1967" {
			t.Errorf("LifeSpanBegin = %q, want 1967", got.GetLifeSpanBegin())
		}
		if len(got.GetAliases()) != 1 || got.GetAliases()[0] != "R.E.O. Speedwagon" {
			t.Errorf("Aliases = %v, want [R.E.O. Speedwagon]", got.GetAliases())
		}
		if svc.gotQuery != "REO Speedwagon" {
			t.Errorf("SearchArtists passed query=%q, want REO Speedwagon", svc.gotQuery)
		}
	})

	t.Run("a genuinely empty result is a valid non-error response", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{returnArtists: nil}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		res, err := h.SearchArtists(context.Background(), connect.NewRequest(&musicv1.SearchMusicBrainzArtistsRequest{Query: "no such artist"}))
		if err != nil {
			t.Fatalf("SearchArtists returned error: %v", err)
		}
		if len(res.Msg.GetArtists()) != 0 {
			t.Fatalf("SearchArtists returned %d artists, want 0", len(res.Msg.GetArtists()))
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{artistsErr: errors.New("boom")}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		_, err := h.SearchArtists(context.Background(), connect.NewRequest(&musicv1.SearchMusicBrainzArtistsRequest{Query: "x"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("SearchArtists with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}

func TestMusicBrainzSearchHandler_ListReleaseGroupsForArtist(t *testing.T) {
	t.Run("valid request returns release groups and passes the mbid through", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{returnArtistReleaseGroups: []ports.ReleaseGroup{
			{ID: "rg-1", Title: "Hi Infidelity", PrimaryType: "Album", FirstReleaseDate: "1980-11-21"},
		}}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		res, err := h.ListReleaseGroupsForArtist(context.Background(), connect.NewRequest(&musicv1.ListMusicBrainzArtistReleaseGroupsRequest{ArtistMbid: "artist-1"}))
		if err != nil {
			t.Fatalf("ListReleaseGroupsForArtist returned error: %v", err)
		}
		if len(res.Msg.GetReleaseGroups()) != 1 {
			t.Fatalf("ListReleaseGroupsForArtist returned %d release groups, want 1", len(res.Msg.GetReleaseGroups()))
		}
		got := res.Msg.GetReleaseGroups()[0]
		if got.GetMbid() != "rg-1" || got.GetTitle() != "Hi Infidelity" {
			t.Errorf("ListReleaseGroupsForArtist returned %+v, want a match for the fake's release group", got)
		}
		if svc.gotArtistMBID != "artist-1" {
			t.Errorf("ListReleaseGroupsForArtist passed artist_mbid=%q, want artist-1", svc.gotArtistMBID)
		}
	})

	t.Run("a genuinely empty result is a valid non-error response", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{returnArtistReleaseGroups: nil}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		res, err := h.ListReleaseGroupsForArtist(context.Background(), connect.NewRequest(&musicv1.ListMusicBrainzArtistReleaseGroupsRequest{ArtistMbid: "artist-1"}))
		if err != nil {
			t.Fatalf("ListReleaseGroupsForArtist returned error: %v", err)
		}
		if len(res.Msg.GetReleaseGroups()) != 0 {
			t.Fatalf("ListReleaseGroupsForArtist returned %d release groups, want 0", len(res.Msg.GetReleaseGroups()))
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{artistReleaseGroupsErr: errors.New("boom")}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		_, err := h.ListReleaseGroupsForArtist(context.Background(), connect.NewRequest(&musicv1.ListMusicBrainzArtistReleaseGroupsRequest{ArtistMbid: "artist-1"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListReleaseGroupsForArtist with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}

func TestMusicBrainzSearchHandler_GetArtist(t *testing.T) {
	t.Run("known mbid returns the artist and passes the mbid through", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{returnArtist: &ports.Artist{
			ID: "artist-1", Name: "REO Speedwagon", SortName: "REO Speedwagon", Type: "Group", Country: "US",
			LifeSpan: ports.LifeSpan{Begin: "1967", End: ""},
			Aliases:  []ports.Alias{{Name: "R.E.O. Speedwagon"}},
		}}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		res, err := h.GetArtist(context.Background(), connect.NewRequest(&musicv1.GetMusicBrainzArtistRequest{Mbid: "artist-1"}))
		if err != nil {
			t.Fatalf("GetArtist returned error: %v", err)
		}
		got := res.Msg.GetArtist()
		if got.GetMbid() != "artist-1" || got.GetName() != "REO Speedwagon" || got.GetType() != "Group" || got.GetCountry() != "US" {
			t.Errorf("GetArtist returned %+v, want a match for the fake's artist", got)
		}
		if svc.gotMBID != "artist-1" {
			t.Errorf("GetArtist passed mbid=%q, want artist-1", svc.gotMBID)
		}
	})

	t.Run("unknown mbid maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{artistErr: ports.ErrNotFound}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		_, err := h.GetArtist(context.Background(), connect.NewRequest(&musicv1.GetMusicBrainzArtistRequest{Mbid: "unknown-mbid"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("GetArtist with an unknown mbid returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	// Regression coverage: GetArtist is the "caller already knows the exact
	// artist" lookup — the right place for isnis/official_url/wikipedia_url/
	// wikidata_url/members, the relations data SearchArtists'
	// MusicBrainzArtist deliberately omits. See the Add Artist flow
	// (band-member Person/EntryPerson creation), Artist Detail's facts
	// sidebar (ISNI, links), and #703's Person-photo Wikidata lookup.
	t.Run("maps isnis, official/wikipedia/wikidata links, and band members from Relations", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{returnArtist: &ports.Artist{
			ID: "artist-1", Name: "REO Speedwagon", Type: "Group",
			ISNIs: []string{"0000000123456789"},
			Relations: []ports.Relation{
				{Type: "official homepage", URL: &ports.RelationURL{Resource: "http://www.speedwagon.com/"}},
				{Type: "wikipedia", URL: &ports.RelationURL{Resource: "https://en.wikipedia.org/wiki/REO_Speedwagon"}},
				{Type: "wikidata", URL: &ports.RelationURL{Resource: "https://www.wikidata.org/wiki/Q845084"}},
				{Type: "allmusic", URL: &ports.RelationURL{Resource: "https://www.allmusic.com/artist/x"}},
				{
					Type: "member of band", Direction: "backward",
					Artist:     &ports.RelationArtist{ID: "member-1", Name: "Kevin Cronin"},
					Attributes: []string{"vocal", "guitar"},
					Begin:      "1972",
				},
				{Type: "member of band", Direction: "backward", Artist: nil}, // malformed edge, no target — must not panic or produce a blank member
			},
		}}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		res, err := h.GetArtist(context.Background(), connect.NewRequest(&musicv1.GetMusicBrainzArtistRequest{Mbid: "artist-1"}))
		if err != nil {
			t.Fatalf("GetArtist returned error: %v", err)
		}

		if len(res.Msg.GetIsnis()) != 1 || res.Msg.GetIsnis()[0] != "0000000123456789" {
			t.Errorf("Isnis = %v, want [0000000123456789]", res.Msg.GetIsnis())
		}
		if res.Msg.GetOfficialUrl() != "http://www.speedwagon.com/" {
			t.Errorf("OfficialUrl = %q, want http://www.speedwagon.com/", res.Msg.GetOfficialUrl())
		}
		if res.Msg.GetWikipediaUrl() != "https://en.wikipedia.org/wiki/REO_Speedwagon" {
			t.Errorf("WikipediaUrl = %q, want https://en.wikipedia.org/wiki/REO_Speedwagon", res.Msg.GetWikipediaUrl())
		}
		if res.Msg.GetWikidataUrl() != "https://www.wikidata.org/wiki/Q845084" {
			t.Errorf("WikidataUrl = %q, want https://www.wikidata.org/wiki/Q845084", res.Msg.GetWikidataUrl())
		}
		if len(res.Msg.GetMembers()) != 1 {
			t.Fatalf("Members = %d, want 1 (the malformed nil-Artist edge must be skipped)", len(res.Msg.GetMembers()))
		}
		m := res.Msg.GetMembers()[0]
		if m.GetMbid() != "member-1" || m.GetName() != "Kevin Cronin" || m.GetBegin() != "1972" {
			t.Errorf("Members[0] = %+v, want mbid=member-1 name=Kevin Cronin begin=1972", m)
		}
		if len(m.GetAttributes()) != 2 {
			t.Errorf("Members[0].Attributes = %v, want [vocal guitar]", m.GetAttributes())
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{artistErr: errors.New("boom")}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		_, err := h.GetArtist(context.Background(), connect.NewRequest(&musicv1.GetMusicBrainzArtistRequest{Mbid: "artist-1"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("GetArtist with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}

func TestMusicBrainzSearchHandler_GetRelease(t *testing.T) {
	t.Run("a multi-medium release returns the summary plus every disc's tracklist", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{returnRelease: &ports.Release{
			ID: "release-1", Title: "Greatest Hits", Country: "US", Date: "1994", Status: "Official",
			Media: []ports.Medium{
				{
					Position: 1, Format: "CD", TrackCount: 2,
					Tracks: []ports.Track{
						{Position: 1, Number: "1", Title: "Roll with the Changes", Length: 245000, Recording: &ports.Recording{ID: "rec-1"}},
						{Position: 2, Number: "2", Title: "Keep on Loving You", Length: 210000}, // no linked recording
					},
				},
				{
					Position: 2, Format: "CD", TrackCount: 1,
					Tracks: []ports.Track{
						{Position: 1, Number: "1", Title: "Can't Fight This Feeling", Length: 300000, Recording: &ports.Recording{ID: "rec-3"}},
					},
				},
			},
		}}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		res, err := h.GetRelease(context.Background(), connect.NewRequest(&musicv1.GetMusicBrainzReleaseRequest{Mbid: "release-1"}))
		if err != nil {
			t.Fatalf("GetRelease returned error: %v", err)
		}
		if svc.gotReleaseMBID != "release-1" {
			t.Errorf("GetRelease passed mbid=%q, want release-1", svc.gotReleaseMBID)
		}

		release := res.Msg.GetRelease()
		if release.GetMbid() != "release-1" || release.GetTitle() != "Greatest Hits" || release.GetMediumCount() != 2 || release.GetTrackCount() != 3 {
			t.Errorf("Release = %+v, want mbid/title/medium_count=2/track_count=3 to match the fake's release", release)
		}

		media := res.Msg.GetMedia()
		if len(media) != 2 {
			t.Fatalf("Media = %d entries, want 2", len(media))
		}
		if len(media[0].GetTracks()) != 2 || len(media[1].GetTracks()) != 1 {
			t.Fatalf("Media track counts = [%d %d], want [2 1]", len(media[0].GetTracks()), len(media[1].GetTracks()))
		}

		t1 := media[0].GetTracks()[0]
		if t1.GetTitle() != "Roll with the Changes" || t1.GetNumber() != "1" || t1.GetLengthMs() != 245000 || t1.GetRecordingMbid() != "rec-1" {
			t.Errorf("Media[0].Tracks[0] = %+v, want a match for the fake's first track", t1)
		}

		// No linked recording — recording_mbid must be "", not a panic.
		t2 := media[0].GetTracks()[1]
		if t2.GetRecordingMbid() != "" {
			t.Errorf("Media[0].Tracks[1].RecordingMbid = %q, want empty (no linked recording)", t2.GetRecordingMbid())
		}
	})

	t.Run("unknown mbid maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeMusicBrainzSearchService{releaseErr: ports.ErrNotFound}
		h := apiconnect.NewMusicBrainzSearchHandler(svc, nil)

		_, err := h.GetRelease(context.Background(), connect.NewRequest(&musicv1.GetMusicBrainzReleaseRequest{Mbid: "unknown-mbid"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("GetRelease with an unknown mbid returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}
