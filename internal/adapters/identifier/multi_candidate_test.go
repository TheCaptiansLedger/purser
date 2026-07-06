package identifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

type stubExtIDRepo struct{}

func (s *stubExtIDRepo) FindEntity(_ context.Context, _, _, _ string) (string, error) {
	return "", errs.ErrNotFound
}

type stubItemRepo2 struct{}

func (s *stubItemRepo2) Get(_ context.Context, _ string) (*domain.Item, error) {
	return nil, errs.ErrNotFound
}

func (s *stubItemRepo2) List(_ context.Context, _ ports.ItemFilter) ([]*domain.Item, int, error) {
	return nil, 0, nil
}

func (s *stubItemRepo2) Save(_ context.Context, _ *domain.Item) error           { return nil }
func (s *stubItemRepo2) Delete(_ context.Context, _ string) error               { return nil }
func (s *stubItemRepo2) DeleteByGroup(_ context.Context, _ string) error        { return nil }
func (s *stubItemRepo2) DeleteByLibraryEntry(_ context.Context, _ string) error { return nil }
func (s *stubItemRepo2) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return nil, errs.ErrNotFound
}

// multiReleaseSource implements ports.MetadataSource and allReleasesLookup.
// Returns a fixed release list for any recording MBID.
type multiReleaseSource struct {
	releases []*domain.ExternalItem
}

func (s *multiReleaseSource) Name() string { return "stub-multi-release" }
func (s *multiReleaseSource) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}
func (s *multiReleaseSource) ImagePriority() int { return 0 }

func (s *multiReleaseSource) FetchAllRecordingReleases(_ context.Context, _ string) ([]*domain.ExternalItem, error) {
	return s.releases, nil
}

// ── TestMultiCandidate ────────────────────────────────────────────────────────

// TestMultiCandidate asserts that one recording MBID with releases in two distinct
// release groups produces exactly two candidates, and that the higher-scoring release
// within the same release group is kept.
func TestMultiCandidate(t *testing.T) {
	const (
		recMBID = "rec-0001"
		rg1     = "rg-0001" // "Hi Infidelity" release group
		rg2     = "rg-0002" // compilation release group
		rel1a   = "rel-1a"  // rg1, exact album match (higher score)
		rel1b   = "rel-1b"  // rg1, prefix match (lower score — should be discarded)
		rel2    = "rel-2"   // rg2, no album overlap
	)

	releases := []*domain.ExternalItem{
		{
			Source: domain.SourceMusicBrainz, ExternalID: recMBID,
			ContentType: domain.ContentTypeMusic, Title: "Take It on the Run", RuntimeSecs: 234,
			GroupExternalID: rg1, GroupTitle: "Hi Infidelity",
			Studio:        &domain.ExternalStudio{Name: "REO Speedwagon"},
			ReleaseDetail: &domain.ExternalReleaseDetail{ReleaseMBID: rel1a, ReleaseTitle: "Hi Infidelity", ReleaseDate: "1981"},
		},
		{
			Source: domain.SourceMusicBrainz, ExternalID: recMBID,
			ContentType: domain.ContentTypeMusic, Title: "Take It on the Run", RuntimeSecs: 234,
			GroupExternalID: rg1, GroupTitle: "Hi Infidelity",
			Studio:        &domain.ExternalStudio{Name: "REO Speedwagon"},
			ReleaseDetail: &domain.ExternalReleaseDetail{ReleaseMBID: rel1b, ReleaseTitle: "Hi Infidelity (2024 Remaster)", ReleaseDate: "2024"},
		},
		{
			Source: domain.SourceMusicBrainz, ExternalID: recMBID,
			ContentType: domain.ContentTypeMusic, Title: "Take It on the Run", RuntimeSecs: 234,
			GroupExternalID: rg2, GroupTitle: "Find Your Own Way Home",
			Studio:        &domain.ExternalStudio{Name: "Various Artists"},
			ReleaseDetail: &domain.ExternalReleaseDetail{ReleaseMBID: rel2, ReleaseTitle: "Find Your Own Way Home", ReleaseDate: "1985"},
		},
	}

	// AcoustID stub: returns one match with score=0.95 for recMBID.
	acoustidSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := acoustidResponse{
			Status: "ok",
			Results: []acoustidResult{{
				ID:    "acoustid-stub",
				Score: 0.95,
				Recordings: []acoustidRecording{{
					ID: recMBID, Title: "Take It on the Run", Duration: 234,
					Artists: []acoustidArtist{{Name: "REO Speedwagon"}},
				}},
			}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer acoustidSrv.Close()

	ac := newAcoustIDClient("test-key", nil)
	ac.baseURL = acoustidSrv.URL

	id := &musicIdentifier{
		extIDs:   &stubExtIDRepo{},
		items:    &stubItemRepo2{},
		sources:  []ports.MetadataSource{&multiReleaseSource{releases: releases}},
		acoustid: ac,
	}

	fp := normalizeFingerprint(&domain.Fingerprint{
		AcoustID: "fake-fingerprint",
		EmbeddedTags: map[string]string{
			"title": "Take It on the Run", "artist": "REO Speedwagon",
			"album": "Hi Infidelity", "duration_ms": "234000",
		},
	})

	candidates, err := id.acoustidStrategy(context.Background(), fp, "/music/test.flac")
	if err != nil {
		t.Fatal(err)
	}

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates (one per release group), got %d", len(candidates))
	}

	for _, c := range candidates {
		if c.MusicDetail == nil {
			t.Error("candidate missing MusicDetail")
			continue
		}
		// rg1 must keep rel1a (exact album match) over rel1b (prefix match).
		if c.MusicDetail.ReleaseGroupMBID == rg1 && c.MusicDetail.ReleaseMBID != rel1a {
			t.Errorf("rg1: expected to keep rel1a (exact match), got %s", c.MusicDetail.ReleaseMBID)
		}
		// All candidates must have three-tier fields populated.
		if c.RecordingConfidence == 0 {
			t.Errorf("RecordingConfidence should not be zero for release group %s", c.MusicDetail.ReleaseGroupMBID)
		}
		if c.ReleaseConfidence == 0 && c.MusicDetail.ReleaseGroupMBID == rg1 {
			t.Errorf("ReleaseConfidence should not be zero for the matching album (rg1)")
		}
	}

	// The rg1 candidate (exact album match) must rank above rg2 (no overlap).
	var rg1Conf, rg2Conf float64
	for _, c := range candidates {
		switch c.MusicDetail.ReleaseGroupMBID {
		case rg1:
			rg1Conf = c.Confidence
		case rg2:
			rg2Conf = c.Confidence
		}
	}
	if rg1Conf <= rg2Conf {
		t.Errorf("Hi Infidelity (%.4f) must outscore compilation (%.4f)", rg1Conf, rg2Conf)
	}
}
