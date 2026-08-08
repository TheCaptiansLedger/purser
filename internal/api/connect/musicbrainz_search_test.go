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
				LabelInfo:    []ports.LabelInfo{{Label: ports.Label{Name: "Epic"}}},
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
