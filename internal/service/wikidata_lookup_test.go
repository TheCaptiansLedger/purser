package service_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// fakeWikidataClient is a minimal ports.WikidataClient double, per
// docs/adr/0003-go-testing-standards.md's "services fake the ports they
// consume" rule.
type fakeWikidataClient struct {
	images    []ports.WikidataImage
	imagesErr error

	gotEntityURL string
}

var _ ports.WikidataClient = (*fakeWikidataClient)(nil)

func (f *fakeWikidataClient) LookupImage(_ context.Context, entityURL string) ([]ports.WikidataImage, error) {
	f.gotEntityURL = entityURL
	if f.imagesErr != nil {
		return nil, f.imagesErr
	}
	return f.images, nil
}

func TestWikidataLookup_LookupImage_PassesEntityURLThrough(t *testing.T) {
	want := []ports.WikidataImage{{URL: "https://commons.wikimedia.org/wiki/Special:FilePath/x.jpg"}}
	wikidata := &fakeWikidataClient{images: want}
	s := service.NewWikidataLookup(wikidata)

	got, err := s.LookupImage(context.Background(), "https://www.wikidata.org/wiki/Q845084")
	if err != nil {
		t.Fatalf("LookupImage returned error: %v", err)
	}
	if len(got) != 1 || got[0] != want[0] {
		t.Errorf("LookupImage = %+v, want %+v", got, want)
	}
	if wikidata.gotEntityURL != "https://www.wikidata.org/wiki/Q845084" {
		t.Errorf("LookupImage called with %q, want the Q845084 entity URL", wikidata.gotEntityURL)
	}
}

func TestWikidataLookup_LookupImage_PropagatesError(t *testing.T) {
	wantErr := errors.New("wikidata unavailable")
	wikidata := &fakeWikidataClient{imagesErr: wantErr}
	s := service.NewWikidataLookup(wikidata)

	if _, err := s.LookupImage(context.Background(), "x"); !errors.Is(err, wantErr) {
		t.Fatalf("LookupImage returned %v, want %v", err, wantErr)
	}
}
