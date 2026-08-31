package service_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// fakeStashDBLookupClient is a minimal ports.StashDBClient double, per
// docs/adr/0003-go-testing-standards.md's "services fake the ports they
// consume" rule. Only LookupPerformer/SearchPerformers are exercised —
// the other methods exist purely to satisfy the interface.
type fakeStashDBLookupClient struct {
	ports.StashDBClient

	performer    *ports.Performer
	performerErr error

	gotID string

	performers []ports.Performer
	searchErr  error

	gotTerm string
}

func (f *fakeStashDBLookupClient) LookupPerformer(_ context.Context, id string) (*ports.Performer, error) {
	f.gotID = id
	if f.performerErr != nil {
		return nil, f.performerErr
	}
	return f.performer, nil
}

func (f *fakeStashDBLookupClient) SearchPerformers(_ context.Context, term string) ([]ports.Performer, error) {
	f.gotTerm = term
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.performers, nil
}

func TestStashDBLookup_LookupPerformer_PassesIDThrough(t *testing.T) {
	want := &ports.Performer{ID: "performer-1", Name: "Jane Doe"}
	stashDB := &fakeStashDBLookupClient{performer: want}
	s := service.NewStashDBLookup(stashDB)

	got, err := s.LookupPerformer(context.Background(), "performer-1")
	if err != nil {
		t.Fatalf("LookupPerformer returned error: %v", err)
	}
	if got != want {
		t.Errorf("LookupPerformer = %+v, want %+v", got, want)
	}
	if stashDB.gotID != "performer-1" {
		t.Errorf("LookupPerformer called with %q, want performer-1", stashDB.gotID)
	}
}

func TestStashDBLookup_LookupPerformer_PropagatesError(t *testing.T) {
	wantErr := errors.New("stashdb unavailable")
	stashDB := &fakeStashDBLookupClient{performerErr: wantErr}
	s := service.NewStashDBLookup(stashDB)

	if _, err := s.LookupPerformer(context.Background(), "x"); !errors.Is(err, wantErr) {
		t.Fatalf("LookupPerformer returned %v, want %v", err, wantErr)
	}
}

func TestStashDBLookup_SearchPerformers_PassesTermThrough(t *testing.T) {
	want := []ports.Performer{{ID: "performer-1", Name: "Alex Coal"}}
	stashDB := &fakeStashDBLookupClient{performers: want}
	s := service.NewStashDBLookup(stashDB)

	got, err := s.SearchPerformers(context.Background(), "Alex Coal")
	if err != nil {
		t.Fatalf("SearchPerformers returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != want[0].ID || got[0].Name != want[0].Name {
		t.Errorf("SearchPerformers = %+v, want %+v", got, want)
	}
	if stashDB.gotTerm != "Alex Coal" {
		t.Errorf("SearchPerformers called with %q, want Alex Coal", stashDB.gotTerm)
	}
}

func TestStashDBLookup_SearchPerformers_PropagatesError(t *testing.T) {
	wantErr := errors.New("stashdb unavailable")
	stashDB := &fakeStashDBLookupClient{searchErr: wantErr}
	s := service.NewStashDBLookup(stashDB)

	if _, err := s.SearchPerformers(context.Background(), "x"); !errors.Is(err, wantErr) {
		t.Fatalf("SearchPerformers returned %v, want %v", err, wantErr)
	}
}
