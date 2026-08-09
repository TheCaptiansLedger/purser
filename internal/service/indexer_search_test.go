package service_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// fakeIndexerSearcher is a minimal ports.IndexerSearcher double — see
// fakeMusicBrainzClient's own doc comment for the convention this follows.
type fakeIndexerSearcher struct {
	releases []ports.IndexerRelease
	err      error

	gotParams ports.IndexerSearchParams
}

var _ ports.IndexerSearcher = (*fakeIndexerSearcher)(nil)

func (f *fakeIndexerSearcher) Search(_ context.Context, params ports.IndexerSearchParams) ([]ports.IndexerRelease, error) {
	f.gotParams = params
	if f.err != nil {
		return nil, f.err
	}
	return f.releases, nil
}

func TestIndexerSearch_Search_PassesParamsThrough(t *testing.T) {
	want := []ports.IndexerRelease{{GUID: "release-1", Title: "Some Release"}}
	fake := &fakeIndexerSearcher{releases: want}
	s := service.NewIndexerSearch(fake)

	params := ports.IndexerSearchParams{Query: "some release", Categories: []int{3000}, IndexerIDs: []int{1}}
	got, err := s.Search(context.Background(), params)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(got) != 1 || got[0].GUID != "release-1" {
		t.Errorf("Search = %+v, want %+v", got, want)
	}
	if fake.gotParams.Query != params.Query || len(fake.gotParams.Categories) != 1 || len(fake.gotParams.IndexerIDs) != 1 {
		t.Errorf("Search called with %+v, want %+v", fake.gotParams, params)
	}
}

func TestIndexerSearch_Search_PropagatesError(t *testing.T) {
	wantErr := errors.New("prowlarr unavailable")
	fake := &fakeIndexerSearcher{err: wantErr}
	s := service.NewIndexerSearch(fake)

	if _, err := s.Search(context.Background(), ports.IndexerSearchParams{Query: "x"}); !errors.Is(err, wantErr) {
		t.Fatalf("Search returned %v, want %v", err, wantErr)
	}
}

func TestIndexerSearch_Search_EmptyResultIsNotAnError(t *testing.T) {
	fake := &fakeIndexerSearcher{releases: []ports.IndexerRelease{}}
	s := service.NewIndexerSearch(fake)

	got, err := s.Search(context.Background(), ports.IndexerSearchParams{Query: "no-such-release"})
	if err != nil {
		t.Fatalf("Search returned error: %v, want nil for a zero-result search", err)
	}
	if len(got) != 0 {
		t.Errorf("Search = %+v, want empty", got)
	}
}
