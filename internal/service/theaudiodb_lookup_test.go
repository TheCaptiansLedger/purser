package service_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// fakeTheAudioDBClient is a minimal ports.TheAudioDBClient double, per
// docs/adr/0003-go-testing-standards.md's "services fake the ports they
// consume" rule.
type fakeTheAudioDBClient struct {
	artist    *ports.TADBArtist
	artistErr error
	album     *ports.TADBAlbum
	albumErr  error

	gotArtistMBID, gotReleaseGroupMBID string
}

var _ ports.TheAudioDBClient = (*fakeTheAudioDBClient)(nil)

func (f *fakeTheAudioDBClient) LookupArtist(_ context.Context, mbid string) (*ports.TADBArtist, error) {
	f.gotArtistMBID = mbid
	if f.artistErr != nil {
		return nil, f.artistErr
	}
	return f.artist, nil
}

func (f *fakeTheAudioDBClient) LookupAlbum(_ context.Context, releaseGroupMBID string) (*ports.TADBAlbum, error) {
	f.gotReleaseGroupMBID = releaseGroupMBID
	if f.albumErr != nil {
		return nil, f.albumErr
	}
	return f.album, nil
}

func TestTheAudioDBLookup_LookupArtist_PassesMBIDThrough(t *testing.T) {
	want := &ports.TADBArtist{MusicBrainzID: "artist-1", Name: "The Beatles"}
	tadb := &fakeTheAudioDBClient{artist: want}
	s := service.NewTheAudioDBLookup(tadb)

	got, err := s.LookupArtist(context.Background(), "artist-1")
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}
	if got != want {
		t.Errorf("LookupArtist = %+v, want %+v", got, want)
	}
	if tadb.gotArtistMBID != "artist-1" {
		t.Errorf("LookupArtist called with %q, want artist-1", tadb.gotArtistMBID)
	}
}

func TestTheAudioDBLookup_LookupArtist_PropagatesError(t *testing.T) {
	wantErr := errors.New("theaudiodb unavailable")
	tadb := &fakeTheAudioDBClient{artistErr: wantErr}
	s := service.NewTheAudioDBLookup(tadb)

	if _, err := s.LookupArtist(context.Background(), "x"); !errors.Is(err, wantErr) {
		t.Fatalf("LookupArtist returned %v, want %v", err, wantErr)
	}
}

func TestTheAudioDBLookup_LookupAlbum_PassesMBIDThrough(t *testing.T) {
	want := &ports.TADBAlbum{MusicBrainzID: "rg-1", Title: "Please Please Me"}
	tadb := &fakeTheAudioDBClient{album: want}
	s := service.NewTheAudioDBLookup(tadb)

	got, err := s.LookupAlbum(context.Background(), "rg-1")
	if err != nil {
		t.Fatalf("LookupAlbum returned error: %v", err)
	}
	if got != want {
		t.Errorf("LookupAlbum = %+v, want %+v", got, want)
	}
	if tadb.gotReleaseGroupMBID != "rg-1" {
		t.Errorf("LookupAlbum called with %q, want rg-1", tadb.gotReleaseGroupMBID)
	}
}

func TestTheAudioDBLookup_LookupAlbum_PropagatesError(t *testing.T) {
	wantErr := errors.New("theaudiodb unavailable")
	tadb := &fakeTheAudioDBClient{albumErr: wantErr}
	s := service.NewTheAudioDBLookup(tadb)

	if _, err := s.LookupAlbum(context.Background(), "rg-1"); !errors.Is(err, wantErr) {
		t.Fatalf("LookupAlbum returned %v, want %v", err, wantErr)
	}
}
