package main

import (
	"context"
	"errors"
	"purser/gen/go/purser/music/v1/musicv1connect"
	"testing"

	"connectrpc.com/connect"

	musicv1 "purser/gen/go/purser/music/v1"
)

// fakeWikidataServiceClient is a minimal musicv1connect.WikidataServiceClient
// double — see fakeFanartTVServiceClient's identical convention.
type fakeWikidataServiceClient struct {
	musicv1connect.WikidataServiceClient

	gotURL string
	resp   *musicv1.LookupWikidataImageResponse
	err    error
}

func (f *fakeWikidataServiceClient) LookupImage(_ context.Context, req *connect.Request[musicv1.LookupWikidataImageRequest]) (*connect.Response[musicv1.LookupWikidataImageResponse], error) {
	f.gotURL = req.Msg.GetUrl()
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.resp), nil
}

func TestRunMusicWikidataLookupImage_PassesURLThroughAndPrints(t *testing.T) {
	client := &fakeWikidataServiceClient{resp: &musicv1.LookupWikidataImageResponse{
		Images: []*musicv1.WikidataImage{{Url: "https://commons.wikimedia.org/wiki/Special:FilePath/REO_Speedwagon.jpg"}},
	}}

	if err := runMusicWikidataLookupImage(context.Background(), client, "https://www.wikidata.org/wiki/Q845084", false); err != nil {
		t.Fatalf("runMusicWikidataLookupImage returned error: %v", err)
	}
	if client.gotURL != "https://www.wikidata.org/wiki/Q845084" {
		t.Errorf("LookupImage called with url=%q, want the Q845084 entity URL", client.gotURL)
	}
}

func TestRunMusicWikidataLookupImage_JSONOutput(t *testing.T) {
	client := &fakeWikidataServiceClient{resp: &musicv1.LookupWikidataImageResponse{
		Images: []*musicv1.WikidataImage{{Url: "https://commons.wikimedia.org/wiki/Special:FilePath/x.jpg"}},
	}}

	if err := runMusicWikidataLookupImage(context.Background(), client, "https://www.wikidata.org/wiki/Q1", true); err != nil {
		t.Fatalf("runMusicWikidataLookupImage returned error: %v", err)
	}
}

func TestRunMusicWikidataLookupImage_PropagatesError(t *testing.T) {
	wantErr := errors.New("wikidata unavailable")
	client := &fakeWikidataServiceClient{err: wantErr}

	if err := runMusicWikidataLookupImage(context.Background(), client, "x", false); !errors.Is(err, wantErr) {
		t.Fatalf("runMusicWikidataLookupImage returned %v, want %v", err, wantErr)
	}
}

func TestNewMusicWikidataCmd_HasLookupImageSubcommand(t *testing.T) {
	cmd := newMusicWikidataCmd()
	if cmd.Name() != "wikidata" {
		t.Errorf("command name = %q, want wikidata", cmd.Name())
	}

	var hasLookupImage bool
	for _, c := range cmd.Commands() {
		if c.Name() == "lookup-image" {
			hasLookupImage = true
		}
	}
	if !hasLookupImage {
		t.Error("wikidata command has no lookup-image subcommand")
	}
}

func TestWikidataImageFields(t *testing.T) {
	fields := wikidataImageFields([]*musicv1.WikidataImage{
		{Url: "https://example.invalid/a.jpg"},
		{Url: "https://example.invalid/b.jpg"},
	})
	if len(fields) != 2 || fields[0].value != "https://example.invalid/a.jpg" {
		t.Errorf("wikidataImageFields = %+v, want 2 fields with the given URLs", fields)
	}
}
