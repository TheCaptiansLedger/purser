package main

import (
	"context"
	"errors"
	"purser/gen/go/purser/music/v1/musicv1connect"
	"testing"

	"connectrpc.com/connect"

	musicv1 "purser/gen/go/purser/music/v1"
)

// fakeFanartTVServiceClient is a minimal musicv1connect.FanartTVServiceClient
// double — see fakeTheAudioDBServiceClient's identical convention.
type fakeFanartTVServiceClient struct {
	musicv1connect.FanartTVServiceClient

	gotMBID string
	resp    *musicv1.LookupFanartTVArtistResponse
	err     error
}

func (f *fakeFanartTVServiceClient) LookupArtist(_ context.Context, req *connect.Request[musicv1.LookupFanartTVArtistRequest]) (*connect.Response[musicv1.LookupFanartTVArtistResponse], error) {
	f.gotMBID = req.Msg.GetMbid()
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.resp), nil
}

func TestRunMusicFanartTVLookupArtist_PassesMBIDThroughAndPrints(t *testing.T) {
	client := &fakeFanartTVServiceClient{resp: &musicv1.LookupFanartTVArtistResponse{
		Artist: &musicv1.FanartTVArtist{Mbid: "artist-1", Name: "The Beatles"},
	}}

	if err := runMusicFanartTVLookupArtist(context.Background(), client, "artist-1", false); err != nil {
		t.Fatalf("runMusicFanartTVLookupArtist returned error: %v", err)
	}
	if client.gotMBID != "artist-1" {
		t.Errorf("LookupArtist called with mbid=%q, want artist-1", client.gotMBID)
	}
}

func TestRunMusicFanartTVLookupArtist_JSONOutput(t *testing.T) {
	client := &fakeFanartTVServiceClient{resp: &musicv1.LookupFanartTVArtistResponse{
		Artist: &musicv1.FanartTVArtist{Mbid: "artist-1", Name: "The Beatles"},
	}}

	if err := runMusicFanartTVLookupArtist(context.Background(), client, "artist-1", true); err != nil {
		t.Fatalf("runMusicFanartTVLookupArtist returned error: %v", err)
	}
}

func TestRunMusicFanartTVLookupArtist_PropagatesError(t *testing.T) {
	wantErr := errors.New("fanarttv unavailable")
	client := &fakeFanartTVServiceClient{err: wantErr}

	if err := runMusicFanartTVLookupArtist(context.Background(), client, "x", false); !errors.Is(err, wantErr) {
		t.Fatalf("runMusicFanartTVLookupArtist returned %v, want %v", err, wantErr)
	}
}

func TestNewMusicFanartTVCmd_HasExpectedSubcommandAndAlias(t *testing.T) {
	cmd := newMusicFanartTVCmd()
	if cmd.Name() != "fanart" {
		t.Errorf("command name = %q, want fanart", cmd.Name())
	}
	found := false
	for _, alias := range cmd.Aliases {
		if alias == "fanarttv" {
			found = true
		}
	}
	if !found {
		t.Errorf("aliases = %v, want to include fanarttv", cmd.Aliases)
	}

	var hasLookupArtist bool
	for _, c := range cmd.Commands() {
		if c.Name() == "lookup-artist" {
			hasLookupArtist = true
		}
	}
	if !hasLookupArtist {
		t.Error("fanart command has no lookup-artist subcommand")
	}
}

func TestCountLabel(t *testing.T) {
	tests := []struct {
		n    int
		unit string
		want string
	}{
		{0, "image(s)", ""},
		{1, "image(s)", "1 image(s)"},
		{122, "release group(s)", "122 release group(s)"},
	}
	for _, tt := range tests {
		if got := countLabel(tt.n, tt.unit); got != tt.want {
			t.Errorf("countLabel(%d, %q) = %q, want %q", tt.n, tt.unit, got, tt.want)
		}
	}
}
