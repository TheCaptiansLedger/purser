package main

import (
	"context"
	"errors"
	"purser/gen/go/purser/music/v1/musicv1connect"
	"testing"

	"connectrpc.com/connect"

	musicv1 "purser/gen/go/purser/music/v1"
)

// fakeTheAudioDBServiceClient is a minimal musicv1connect.TheAudioDBServiceClient
// double, exercising only the RPCs runMusicTheAudioDBLookupArtist/Album
// actually call — same convention fakeLibraryEntryClient
// (seed_various_artists_test.go) already follows.
type fakeTheAudioDBServiceClient struct {
	musicv1connect.TheAudioDBServiceClient

	gotArtistMBID string
	artistResp    *musicv1.LookupTheAudioDBArtistResponse
	artistErr     error

	gotReleaseGroupMBID string
	albumResp           *musicv1.LookupTheAudioDBAlbumResponse
	albumErr            error
}

func (f *fakeTheAudioDBServiceClient) LookupArtist(_ context.Context, req *connect.Request[musicv1.LookupTheAudioDBArtistRequest]) (*connect.Response[musicv1.LookupTheAudioDBArtistResponse], error) {
	f.gotArtistMBID = req.Msg.GetMbid()
	if f.artistErr != nil {
		return nil, f.artistErr
	}
	return connect.NewResponse(f.artistResp), nil
}

func (f *fakeTheAudioDBServiceClient) LookupAlbum(_ context.Context, req *connect.Request[musicv1.LookupTheAudioDBAlbumRequest]) (*connect.Response[musicv1.LookupTheAudioDBAlbumResponse], error) {
	f.gotReleaseGroupMBID = req.Msg.GetReleaseGroupMbid()
	if f.albumErr != nil {
		return nil, f.albumErr
	}
	return connect.NewResponse(f.albumResp), nil
}

func TestRunMusicTheAudioDBLookupArtist_PassesMBIDThroughAndPrints(t *testing.T) {
	client := &fakeTheAudioDBServiceClient{artistResp: &musicv1.LookupTheAudioDBArtistResponse{
		Artist: &musicv1.TheAudioDBArtist{Mbid: "artist-1", Name: "The Beatles"},
	}}

	if err := runMusicTheAudioDBLookupArtist(context.Background(), client, "artist-1", false); err != nil {
		t.Fatalf("runMusicTheAudioDBLookupArtist returned error: %v", err)
	}
	if client.gotArtistMBID != "artist-1" {
		t.Errorf("LookupArtist called with mbid=%q, want artist-1", client.gotArtistMBID)
	}
}

func TestRunMusicTheAudioDBLookupArtist_JSONOutput(t *testing.T) {
	client := &fakeTheAudioDBServiceClient{artistResp: &musicv1.LookupTheAudioDBArtistResponse{
		Artist: &musicv1.TheAudioDBArtist{Mbid: "artist-1", Name: "The Beatles"},
	}}

	if err := runMusicTheAudioDBLookupArtist(context.Background(), client, "artist-1", true); err != nil {
		t.Fatalf("runMusicTheAudioDBLookupArtist returned error: %v", err)
	}
}

func TestRunMusicTheAudioDBLookupArtist_PropagatesError(t *testing.T) {
	wantErr := errors.New("theaudiodb unavailable")
	client := &fakeTheAudioDBServiceClient{artistErr: wantErr}

	if err := runMusicTheAudioDBLookupArtist(context.Background(), client, "x", false); !errors.Is(err, wantErr) {
		t.Fatalf("runMusicTheAudioDBLookupArtist returned %v, want %v", err, wantErr)
	}
}

func TestRunMusicTheAudioDBLookupAlbum_PassesMBIDThroughAndPrints(t *testing.T) {
	client := &fakeTheAudioDBServiceClient{albumResp: &musicv1.LookupTheAudioDBAlbumResponse{
		Album: &musicv1.TheAudioDBAlbum{Mbid: "rg-1", Title: "Please Please Me"},
	}}

	if err := runMusicTheAudioDBLookupAlbum(context.Background(), client, "rg-1", false); err != nil {
		t.Fatalf("runMusicTheAudioDBLookupAlbum returned error: %v", err)
	}
	if client.gotReleaseGroupMBID != "rg-1" {
		t.Errorf("LookupAlbum called with release_group_mbid=%q, want rg-1", client.gotReleaseGroupMBID)
	}
}

func TestRunMusicTheAudioDBLookupAlbum_PropagatesError(t *testing.T) {
	wantErr := errors.New("theaudiodb unavailable")
	client := &fakeTheAudioDBServiceClient{albumErr: wantErr}

	if err := runMusicTheAudioDBLookupAlbum(context.Background(), client, "rg-1", false); !errors.Is(err, wantErr) {
		t.Fatalf("runMusicTheAudioDBLookupAlbum returned %v, want %v", err, wantErr)
	}
}

func TestNewMusicTheAudioDBCmd_HasExpectedSubcommands(t *testing.T) {
	cmd := newMusicTheAudioDBCmd()
	names := map[string]bool{}
	for _, c := range cmd.Commands() {
		names[c.Name()] = true
	}
	if !names["lookup-artist"] || !names["lookup-album"] {
		t.Errorf("theaudiodb command has subcommands %v, want lookup-artist and lookup-album", names)
	}
}
