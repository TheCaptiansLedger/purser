package service_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// fakeThePornDBLookupClient is a minimal ports.ThePornDBClient double, per
// docs/adr/0003-go-testing-standards.md's "services fake the ports they
// consume" rule. Only LookupPerformer/SearchPerformers are exercised —
// the other methods exist purely to satisfy the interface.
type fakeThePornDBLookupClient struct {
	ports.ThePornDBClient

	performer    *ports.TPDBPerformer
	performerErr error

	gotID string

	performers []ports.TPDBPerformer
	searchErr  error

	gotTerm string
}

func (f *fakeThePornDBLookupClient) LookupPerformer(_ context.Context, id string) (*ports.TPDBPerformer, error) {
	f.gotID = id
	if f.performerErr != nil {
		return nil, f.performerErr
	}
	return f.performer, nil
}

func (f *fakeThePornDBLookupClient) SearchPerformers(_ context.Context, term string) ([]ports.TPDBPerformer, error) {
	f.gotTerm = term
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return f.performers, nil
}

func TestThePornDBLookup_LookupPerformer_PassesIDThrough(t *testing.T) {
	want := &ports.TPDBPerformer{ID: "performer-1", Name: "Jane Doe"}
	tpdb := &fakeThePornDBLookupClient{performer: want}
	s := service.NewThePornDBLookup(tpdb)

	got, err := s.LookupPerformer(context.Background(), "performer-1")
	if err != nil {
		t.Fatalf("LookupPerformer returned error: %v", err)
	}
	if got != want {
		t.Errorf("LookupPerformer = %+v, want %+v", got, want)
	}
	if tpdb.gotID != "performer-1" {
		t.Errorf("LookupPerformer called with %q, want performer-1", tpdb.gotID)
	}
}

func TestThePornDBLookup_LookupPerformer_PropagatesError(t *testing.T) {
	wantErr := errors.New("theporndb unavailable")
	tpdb := &fakeThePornDBLookupClient{performerErr: wantErr}
	s := service.NewThePornDBLookup(tpdb)

	if _, err := s.LookupPerformer(context.Background(), "x"); !errors.Is(err, wantErr) {
		t.Fatalf("LookupPerformer returned %v, want %v", err, wantErr)
	}
}

func TestThePornDBLookup_SearchPerformers_PassesTermThrough(t *testing.T) {
	want := []ports.TPDBPerformer{{ID: "performer-1", Name: "Alex Coal"}}
	tpdb := &fakeThePornDBLookupClient{performers: want}
	s := service.NewThePornDBLookup(tpdb)

	got, err := s.SearchPerformers(context.Background(), "Alex Coal")
	if err != nil {
		t.Fatalf("SearchPerformers returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != want[0].ID || got[0].Name != want[0].Name {
		t.Errorf("SearchPerformers = %+v, want %+v", got, want)
	}
	if tpdb.gotTerm != "Alex Coal" {
		t.Errorf("SearchPerformers called with %q, want Alex Coal", tpdb.gotTerm)
	}
}

func TestThePornDBLookup_SearchPerformers_PropagatesError(t *testing.T) {
	wantErr := errors.New("theporndb unavailable")
	tpdb := &fakeThePornDBLookupClient{searchErr: wantErr}
	s := service.NewThePornDBLookup(tpdb)

	if _, err := s.SearchPerformers(context.Background(), "x"); !errors.Is(err, wantErr) {
		t.Fatalf("SearchPerformers returned %v, want %v", err, wantErr)
	}
}
