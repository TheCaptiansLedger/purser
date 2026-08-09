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

type fakeFanartTVLookupService struct {
	gotMBID      string
	returnArtist *ports.FanartArtist
	artistErr    error
}

func (f *fakeFanartTVLookupService) LookupArtist(_ context.Context, mbid string) (*ports.FanartArtist, error) {
	f.gotMBID = mbid
	if f.artistErr != nil {
		return nil, f.artistErr
	}
	return f.returnArtist, nil
}

func TestFanartTVHandler_LookupArtist(t *testing.T) {
	t.Run("valid request maps artist images and albums correctly", func(t *testing.T) {
		svc := &fakeFanartTVLookupService{returnArtist: &ports.FanartArtist{
			Name:               "The Beatles",
			MBID:               "artist-1",
			ArtistThumb:        []ports.FanartImage{{ID: "1", URL: "https://example.invalid/thumb.jpg", Likes: "11"}},
			Artist4KBackground: []ports.FanartImage{{ID: "2", URL: "https://example.invalid/4k.jpg", Likes: "1"}},
			Albums: map[string]ports.FanartAlbumImages{
				"rg-1": {
					AlbumCover: []ports.FanartImage{{ID: "3", URL: "https://example.invalid/cover.jpg", Likes: "8"}},
					CDArt:      []ports.FanartCDArt{{ID: "4", URL: "https://example.invalid/cdart.png", Likes: "8", Disc: "1", Size: "1000"}},
				},
			},
		}}
		h := apiconnect.NewFanartTVHandler(svc, nil)

		res, err := h.LookupArtist(context.Background(), connect.NewRequest(&musicv1.LookupFanartTVArtistRequest{Mbid: "artist-1"}))
		if err != nil {
			t.Fatalf("LookupArtist returned error: %v", err)
		}
		got := res.Msg.GetArtist()
		if got.GetName() != "The Beatles" || got.GetMbid() != "artist-1" {
			t.Errorf("LookupArtist = %+v, want Name=The Beatles Mbid=artist-1", got)
		}
		if len(got.GetArtistThumb()) != 1 || got.GetArtistThumb()[0].GetUrl() != "https://example.invalid/thumb.jpg" {
			t.Errorf("ArtistThumb = %+v, want one entry with the fake's URL", got.GetArtistThumb())
		}
		if len(got.GetArtist_4KBackground()) != 1 {
			t.Errorf("Artist_4KBackground = %+v, want one entry", got.GetArtist_4KBackground())
		}
		album, ok := got.GetAlbums()["rg-1"]
		if !ok {
			t.Fatalf("Albums = %+v, want a rg-1 entry", got.GetAlbums())
		}
		if len(album.GetAlbumCover()) != 1 || len(album.GetCdArt()) != 1 || album.GetCdArt()[0].GetDisc() != "1" {
			t.Errorf("Albums[rg-1] = %+v, want one cover and one disc-1 cdart entry", album)
		}
		if svc.gotMBID != "artist-1" {
			t.Errorf("LookupArtist passed mbid=%q, want artist-1", svc.gotMBID)
		}
	})

	t.Run("ErrNotFound maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeFanartTVLookupService{artistErr: ports.ErrNotFound}
		h := apiconnect.NewFanartTVHandler(svc, nil)

		_, err := h.LookupArtist(context.Background(), connect.NewRequest(&musicv1.LookupFanartTVArtistRequest{Mbid: "x"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("LookupArtist with ports.ErrNotFound returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})

	t.Run("an unmapped error maps to CodeInternal", func(t *testing.T) {
		svc := &fakeFanartTVLookupService{artistErr: errors.New("boom")}
		h := apiconnect.NewFanartTVHandler(svc, nil)

		_, err := h.LookupArtist(context.Background(), connect.NewRequest(&musicv1.LookupFanartTVArtistRequest{Mbid: "x"}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("LookupArtist with an unmapped error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})

	t.Run("no albums maps to an empty, non-nil map", func(t *testing.T) {
		svc := &fakeFanartTVLookupService{returnArtist: &ports.FanartArtist{Name: "Solo Artist", MBID: "artist-2"}}
		h := apiconnect.NewFanartTVHandler(svc, nil)

		res, err := h.LookupArtist(context.Background(), connect.NewRequest(&musicv1.LookupFanartTVArtistRequest{Mbid: "artist-2"}))
		if err != nil {
			t.Fatalf("LookupArtist returned error: %v", err)
		}
		if len(res.Msg.GetArtist().GetAlbums()) != 0 {
			t.Errorf("Albums = %+v, want empty", res.Msg.GetArtist().GetAlbums())
		}
	})
}
