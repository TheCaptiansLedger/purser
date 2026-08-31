package main

import (
	"context"
	"errors"
	"purser/gen/go/purser/afterdark/v1/afterdarkv1connect"
	"testing"

	"connectrpc.com/connect"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
)

// fakeStashDBServiceClient is a minimal afterdarkv1connect.StashDBServiceClient
// double — see fakeFanartTVServiceClient's identical convention.
type fakeStashDBServiceClient struct {
	afterdarkv1connect.StashDBServiceClient

	gotID string
	resp  *afterdarkv1.LookupStashDBPerformerResponse
	err   error

	gotTerm    string
	searchResp *afterdarkv1.SearchStashDBPerformersResponse
	searchErr  error
}

func (f *fakeStashDBServiceClient) LookupPerformer(_ context.Context, req *connect.Request[afterdarkv1.LookupStashDBPerformerRequest]) (*connect.Response[afterdarkv1.LookupStashDBPerformerResponse], error) {
	f.gotID = req.Msg.GetId()
	if f.err != nil {
		return nil, f.err
	}
	return connect.NewResponse(f.resp), nil
}

func (f *fakeStashDBServiceClient) SearchPerformers(_ context.Context, req *connect.Request[afterdarkv1.SearchStashDBPerformersRequest]) (*connect.Response[afterdarkv1.SearchStashDBPerformersResponse], error) {
	f.gotTerm = req.Msg.GetTerm()
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return connect.NewResponse(f.searchResp), nil
}

func TestRunAfterDarkStashDBLookupPerformer_PassesIDThroughAndPrints(t *testing.T) {
	client := &fakeStashDBServiceClient{resp: &afterdarkv1.LookupStashDBPerformerResponse{
		Performer: &afterdarkv1.StashDBPerformer{Id: "performer-1", Name: "Jane Doe"},
	}}

	if err := runAfterDarkStashDBLookupPerformer(context.Background(), client, "performer-1", false); err != nil {
		t.Fatalf("runAfterDarkStashDBLookupPerformer returned error: %v", err)
	}
	if client.gotID != "performer-1" {
		t.Errorf("LookupPerformer called with id=%q, want performer-1", client.gotID)
	}
}

func TestRunAfterDarkStashDBLookupPerformer_JSONOutput(t *testing.T) {
	client := &fakeStashDBServiceClient{resp: &afterdarkv1.LookupStashDBPerformerResponse{
		Performer: &afterdarkv1.StashDBPerformer{Id: "performer-1", Name: "Jane Doe"},
	}}

	if err := runAfterDarkStashDBLookupPerformer(context.Background(), client, "performer-1", true); err != nil {
		t.Fatalf("runAfterDarkStashDBLookupPerformer returned error: %v", err)
	}
}

func TestRunAfterDarkStashDBLookupPerformer_PropagatesError(t *testing.T) {
	wantErr := errors.New("stashdb unavailable")
	client := &fakeStashDBServiceClient{err: wantErr}

	if err := runAfterDarkStashDBLookupPerformer(context.Background(), client, "x", false); !errors.Is(err, wantErr) {
		t.Fatalf("runAfterDarkStashDBLookupPerformer returned %v, want %v", err, wantErr)
	}
}

func TestNewAfterDarkStashDBCmd_HasLookupPerformerSubcommand(t *testing.T) {
	cmd := newAfterDarkStashDBCmd()
	if cmd.Name() != "stashdb" {
		t.Errorf("command name = %q, want stashdb", cmd.Name())
	}

	var hasLookupPerformer bool
	for _, c := range cmd.Commands() {
		if c.Name() == "lookup-performer" {
			hasLookupPerformer = true
		}
	}
	if !hasLookupPerformer {
		t.Error("stashdb command has no lookup-performer subcommand")
	}

	var hasSearchPerformers bool
	for _, c := range cmd.Commands() {
		if c.Name() == "search-performers" {
			hasSearchPerformers = true
		}
	}
	if !hasSearchPerformers {
		t.Error("stashdb command has no search-performers subcommand")
	}
}

func TestRunAfterDarkStashDBSearchPerformers_PassesTermThroughAndPrints(t *testing.T) {
	client := &fakeStashDBServiceClient{searchResp: &afterdarkv1.SearchStashDBPerformersResponse{
		Performers: []*afterdarkv1.StashDBPerformer{{Id: "performer-1", Name: "Alex Coal"}},
	}}

	if err := runAfterDarkStashDBSearchPerformers(context.Background(), client, "Alex Coal", false); err != nil {
		t.Fatalf("runAfterDarkStashDBSearchPerformers returned error: %v", err)
	}
	if client.gotTerm != "Alex Coal" {
		t.Errorf("SearchPerformers called with term=%q, want Alex Coal", client.gotTerm)
	}
}

func TestRunAfterDarkStashDBSearchPerformers_PropagatesError(t *testing.T) {
	wantErr := errors.New("stashdb unavailable")
	client := &fakeStashDBServiceClient{searchErr: wantErr}

	if err := runAfterDarkStashDBSearchPerformers(context.Background(), client, "x", false); !errors.Is(err, wantErr) {
		t.Fatalf("runAfterDarkStashDBSearchPerformers returned %v, want %v", err, wantErr)
	}
}

func TestStashDBPerformerSearchFields(t *testing.T) {
	fields := stashDBPerformerSearchFields([]*afterdarkv1.StashDBPerformer{
		{Id: "performer-1", Name: "Alex Coal", Images: []*afterdarkv1.StashDBImage{{Url: "https://example.invalid/a.jpg"}}},
	})
	if len(fields) != 1 || fields[0].label != "Result 1" {
		t.Errorf("stashDBPerformerSearchFields = %+v, want 1 field labeled Result 1", fields)
	}
}

func TestStashDBPerformerFields(t *testing.T) {
	fields := stashDBPerformerFields(&afterdarkv1.StashDBPerformer{
		Name:    "Jane Doe",
		Id:      "performer-1",
		Gender:  "FEMALE",
		Country: "US",
		Images:  []*afterdarkv1.StashDBImage{{Url: "https://example.invalid/a.jpg"}},
	})
	if len(fields) != 6 || fields[0].value != "Jane Doe" {
		t.Errorf("stashDBPerformerFields = %+v, want 6 fields starting with Jane Doe", fields)
	}
}
