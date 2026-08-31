package main

import (
	"context"
	"errors"
	"purser/gen/go/purser/afterdark/v1/afterdarkv1connect"
	"testing"

	"connectrpc.com/connect"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
)

// fakeThePornDBServiceClient is a minimal afterdarkv1connect.ThePornDBServiceClient
// double — see fakeStashDBServiceClient's identical convention.
type fakeThePornDBServiceClient struct {
	afterdarkv1connect.ThePornDBServiceClient

	gotID string
	resp  *afterdarkv1.LookupThePornDBPerformerResponse
	err   error

	gotTerm    string
	searchResp *afterdarkv1.SearchThePornDBPerformersResponse
	searchErr  error
}

func (f *fakeThePornDBServiceClient) LookupPerformer(_ context.Context, req *connect.Request[afterdarkv1.LookupThePornDBPerformerRequest]) (*connect.Response[afterdarkv1.LookupThePornDBPerformerResponse], error) {
	f.gotID = req.Msg.GetId()
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.resp), nil
}

func (f *fakeThePornDBServiceClient) SearchPerformers(_ context.Context, req *connect.Request[afterdarkv1.SearchThePornDBPerformersRequest]) (*connect.Response[afterdarkv1.SearchThePornDBPerformersResponse], error) {
	f.gotTerm = req.Msg.GetTerm()
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return connect.NewResponse(f.searchResp), nil
}

func TestRunAfterDarkThePornDBLookupPerformer_PassesIDThroughAndPrints(t *testing.T) {
	client := &fakeThePornDBServiceClient{resp: &afterdarkv1.LookupThePornDBPerformerResponse{
		Performer: &afterdarkv1.TPDBPerformer{Id: "performer-1", Name: "Jane Doe"},
	}}

	if err := runAfterDarkThePornDBLookupPerformer(context.Background(), client, "performer-1", false); err != nil {
		t.Fatalf("runAfterDarkThePornDBLookupPerformer returned error: %v", err)
	}
	if client.gotID != "performer-1" {
		t.Errorf("LookupPerformer called with id=%q, want performer-1", client.gotID)
	}
}

func TestRunAfterDarkThePornDBLookupPerformer_JSONOutput(t *testing.T) {
	client := &fakeThePornDBServiceClient{resp: &afterdarkv1.LookupThePornDBPerformerResponse{
		Performer: &afterdarkv1.TPDBPerformer{Id: "performer-1", Name: "Jane Doe"},
	}}

	if err := runAfterDarkThePornDBLookupPerformer(context.Background(), client, "performer-1", true); err != nil {
		t.Fatalf("runAfterDarkThePornDBLookupPerformer returned error: %v", err)
	}
}

func TestRunAfterDarkThePornDBLookupPerformer_PropagatesError(t *testing.T) {
	wantErr := errors.New("theporndb unavailable")
	client := &fakeThePornDBServiceClient{err: wantErr}

	if err := runAfterDarkThePornDBLookupPerformer(context.Background(), client, "x", false); !errors.Is(err, wantErr) {
		t.Fatalf("runAfterDarkThePornDBLookupPerformer returned %v, want %v", err, wantErr)
	}
}

func TestNewAfterDarkThePornDBCmd_HasLookupPerformerSubcommandAndAlias(t *testing.T) {
	cmd := newAfterDarkThePornDBCmd()
	if cmd.Name() != "theporndb" {
		t.Errorf("command name = %q, want theporndb", cmd.Name())
	}
	found := false
	for _, alias := range cmd.Aliases {
		if alias == "tpdb" {
			found = true
		}
	}
	if !found {
		t.Errorf("aliases = %v, want to include tpdb", cmd.Aliases)
	}

	var hasLookupPerformer bool
	for _, c := range cmd.Commands() {
		if c.Name() == "lookup-performer" {
			hasLookupPerformer = true
		}
	}
	if !hasLookupPerformer {
		t.Error("theporndb command has no lookup-performer subcommand")
	}

	var hasSearchPerformers bool
	for _, c := range cmd.Commands() {
		if c.Name() == "search-performers" {
			hasSearchPerformers = true
		}
	}
	if !hasSearchPerformers {
		t.Error("theporndb command has no search-performers subcommand")
	}
}

func TestRunAfterDarkThePornDBSearchPerformers_PassesTermThroughAndPrints(t *testing.T) {
	client := &fakeThePornDBServiceClient{searchResp: &afterdarkv1.SearchThePornDBPerformersResponse{
		Performers: []*afterdarkv1.TPDBPerformer{{Id: "performer-1", Name: "Alex Coal"}},
	}}

	if err := runAfterDarkThePornDBSearchPerformers(context.Background(), client, "Alex Coal", false); err != nil {
		t.Fatalf("runAfterDarkThePornDBSearchPerformers returned error: %v", err)
	}
	if client.gotTerm != "Alex Coal" {
		t.Errorf("SearchPerformers called with term=%q, want Alex Coal", client.gotTerm)
	}
}

func TestRunAfterDarkThePornDBSearchPerformers_PropagatesError(t *testing.T) {
	wantErr := errors.New("theporndb unavailable")
	client := &fakeThePornDBServiceClient{searchErr: wantErr}

	if err := runAfterDarkThePornDBSearchPerformers(context.Background(), client, "x", false); !errors.Is(err, wantErr) {
		t.Fatalf("runAfterDarkThePornDBSearchPerformers returned %v, want %v", err, wantErr)
	}
}

func TestTPDBPerformerSearchFields(t *testing.T) {
	fields := tpdbPerformerSearchFields([]*afterdarkv1.TPDBPerformer{
		{Id: "performer-1", Name: "Alex Coal", Posters: []*afterdarkv1.TPDBImage{{Url: "https://example.invalid/poster.jpg"}}},
	})
	if len(fields) != 1 || fields[0].label != "Result 1" {
		t.Errorf("tpdbPerformerSearchFields = %+v, want 1 field labeled Result 1", fields)
	}
}

func TestTPDBPerformerFields(t *testing.T) {
	fields := tpdbPerformerFields(&afterdarkv1.TPDBPerformer{
		Name:      "Jane Doe",
		Id:        "performer-1",
		Image:     "https://example.invalid/full.jpg",
		Thumbnail: "https://example.invalid/thumb.jpg",
		Posters:   []*afterdarkv1.TPDBImage{{Url: "https://example.invalid/poster.jpg"}},
	})
	if len(fields) != 5 || fields[0].value != "Jane Doe" {
		t.Errorf("tpdbPerformerFields = %+v, want 5 fields starting with Jane Doe", fields)
	}
}
