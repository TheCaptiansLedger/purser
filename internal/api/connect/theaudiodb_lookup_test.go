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

type fakeTheAudioDBLookupService struct {
	gotArtistMBID string
	returnArtist  *ports.TADBArtist
	artistErr     error

	gotReleaseGroupMBID string
	returnAlbum         *ports.TADBAlbum
	albumErr            error
}

func (f *fakeTheAudioDBLookupService) LookupArtist(_ context.Context, mbid string) (*ports.TADBArtist, error) {
	f.gotArtistMBID = mbid
	if f.artistErr != nil {
		return nil, f.artistErr
	}
	return f.returnArtist, nil
}

func (f *fakeTheAudioDBLookupService) LookupAlbum(_ context.Context, releaseGroupMBID string) (*ports.TADBAlbum, error) {
	f.gotReleaseGroupMBID = releaseGroupMBID
	if f.albumErr != nil {
		return nil, f.albumErr
	}
	return f.returnAlbum, nil
}

func TestTheAudioDBHandler_LookupArtist(t *testing.T) {
	t.Run("valid request returns the artist mapped correctly", func(t *testing.T) {
		svc := &fakeTheAudioDBLookupService{returnArtist: &ports.TADBArtist{
			MusicBrainzID: "artist-1",
			Name:          "The Beatles",
			Thumb:         "https://example.invalid/thumb.jpg",
			Members:       "4",
		}}
		h := apiconnect.NewTheAudioDBHandler(svc, nil)

		res, err := h.LookupArtist(context.Background(), connect.NewRequest(&musicv1.LookupTheAudioDBArtistRequest{Mbid: "artist-1"}))
		if err != nil {
			t.Fatalf("LookupArtist returned error: %v", err)
		}
		got := res.Msg.GetArtist()
		if got.GetMbid() != "artist-1" || got.GetName() != "The Beatles" || got.GetThumb() != "https://example.invalid/thumb.jpg" || got.GetMembers() != "4" {
			t.Errorf("LookupArtist = %+v, want a match for the fake's artist", got)
		}
		if svc.gotArtistMBID != "artist-1" {
			t.Errorf("LookupArtist passed mbid=%q, want artist-1", svc.gotArtistMBID)
		}
	})

	t.Run("ErrNotFound maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeTheAudioDBLookupService{artistErr: ports.ErrNotFound}
		h := apiconnect.NewTheAudioDBHandler(svc, nil)

		_, err := h.LookupArtist(context.Background(), connect.NewRequest(&musicv1.LookupTheAudioDBArtistRequest{Mbid: "x"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("LookupArtist with ports.ErrNotFound returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeTheAudioDBLookupService{artistErr: errors.New("boom")}
		h := apiconnect.NewTheAudioDBHandler(svc, nil)

		_, err := h.LookupArtist(context.Background(), connect.NewRequest(&musicv1.LookupTheAudioDBArtistRequest{Mbid: "x"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("LookupArtist with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}

func TestTheAudioDBHandler_LookupAlbum(t *testing.T) {
	t.Run("valid request returns the album mapped correctly", func(t *testing.T) {
		svc := &fakeTheAudioDBLookupService{returnAlbum: &ports.TADBAlbum{
			MusicBrainzID:       "rg-1",
			MusicBrainzArtistID: "artist-1",
			Title:               "Please Please Me",
			Thumb:               "https://example.invalid/album-thumb.jpg",
		}}
		h := apiconnect.NewTheAudioDBHandler(svc, nil)

		res, err := h.LookupAlbum(context.Background(), connect.NewRequest(&musicv1.LookupTheAudioDBAlbumRequest{ReleaseGroupMbid: "rg-1"}))
		if err != nil {
			t.Fatalf("LookupAlbum returned error: %v", err)
		}
		got := res.Msg.GetAlbum()
		if got.GetMbid() != "rg-1" || got.GetArtistMbid() != "artist-1" || got.GetTitle() != "Please Please Me" || got.GetThumb() != "https://example.invalid/album-thumb.jpg" {
			t.Errorf("LookupAlbum = %+v, want a match for the fake's album", got)
		}
		if svc.gotReleaseGroupMBID != "rg-1" {
			t.Errorf("LookupAlbum passed release_group_mbid=%q, want rg-1", svc.gotReleaseGroupMBID)
		}
	})

	t.Run("ErrNotFound maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeTheAudioDBLookupService{albumErr: ports.ErrNotFound}
		h := apiconnect.NewTheAudioDBHandler(svc, nil)

		_, err := h.LookupAlbum(context.Background(), connect.NewRequest(&musicv1.LookupTheAudioDBAlbumRequest{ReleaseGroupMbid: "x"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("LookupAlbum with ports.ErrNotFound returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}
