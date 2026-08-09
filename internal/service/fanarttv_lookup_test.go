package service_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// fakeFanartTVClient is a minimal ports.FanartTVClient double, per
// docs/adr/0003-go-testing-standards.md's "services fake the ports they
// consume" rule.
type fakeFanartTVClient struct {
	artist    *ports.FanartArtist
	artistErr error

	gotMBID string
}

var _ ports.FanartTVClient = (*fakeFanartTVClient)(nil)

func (f *fakeFanartTVClient) LookupArtist(_ context.Context, mbid string) (*ports.FanartArtist, error) {
	f.gotMBID = mbid
	if f.artistErr != nil {
		return nil, f.artistErr
	}
	return f.artist, nil
}

func TestFanartTVLookup_LookupArtist_PassesMBIDThrough(t *testing.T) {
	want := &ports.FanartArtist{MBID: "artist-1", Name: "The Beatles"}
	fanart := &fakeFanartTVClient{artist: want}
	s := service.NewFanartTVLookup(fanart)

	got, err := s.LookupArtist(context.Background(), "artist-1")
	if err != nil {
		t.Fatalf("LookupArtist returned error: %v", err)
	}
	if got != want {
		t.Errorf("LookupArtist = %+v, want %+v", got, want)
	}
	if fanart.gotMBID != "artist-1" {
		t.Errorf("LookupArtist called with %q, want artist-1", fanart.gotMBID)
	}
}

func TestFanartTVLookup_LookupArtist_PropagatesError(t *testing.T) {
	wantErr := errors.New("fanarttv unavailable")
	fanart := &fakeFanartTVClient{artistErr: wantErr}
	s := service.NewFanartTVLookup(fanart)

	if _, err := s.LookupArtist(context.Background(), "x"); !errors.Is(err, wantErr) {
		t.Fatalf("LookupArtist returned %v, want %v", err, wantErr)
	}
}
