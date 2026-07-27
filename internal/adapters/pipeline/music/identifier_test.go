package music_test

import (
	"context"
	"purser/internal/adapters/pipeline/music"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"testing"
)

// fakeMusicBrainz is a hand-rolled ports.MusicBrainzClient double, per
// docs/adr/0003-go-testing-standards.md's "services fake the ports they
// consume" rule — no HTTP fixture server needed for logic tests. Every
// unconfigured lookup returns ports.ErrNotFound / an empty slice, matching
// the real adapter's documented "empty is not an error" search semantics.
type fakeMusicBrainz struct {
	releaseGroups        map[string]ports.ReleaseGroup
	releaseGroupsByQuery map[string][]ports.ReleaseGroup // key: artist+"|"+album
	releases             map[string]ports.Release
	releasesByRG         map[string][]ports.Release
	releasesByBarcode    map[string][]ports.Release
	recordingsByISRC     map[string][]ports.Recording

	lookupReleaseCalls      int
	searchReleaseGroupCalls int
}

var _ ports.MusicBrainzClient = (*fakeMusicBrainz)(nil)

func newFakeMusicBrainz() *fakeMusicBrainz {
	return &fakeMusicBrainz{
		releaseGroups:        map[string]ports.ReleaseGroup{},
		releaseGroupsByQuery: map[string][]ports.ReleaseGroup{},
		releases:             map[string]ports.Release{},
		releasesByRG:         map[string][]ports.Release{},
		releasesByBarcode:    map[string][]ports.Release{},
		recordingsByISRC:     map[string][]ports.Recording{},
	}
}

func (f *fakeMusicBrainz) LookupArtist(_ context.Context, _ string) (*ports.Artist, error) {
	return nil, ports.ErrNotFound
}

func (f *fakeMusicBrainz) SearchArtists(_ context.Context, _ string) ([]ports.Artist, error) {
	return nil, nil
}

func (f *fakeMusicBrainz) LookupReleaseGroup(_ context.Context, mbid string) (*ports.ReleaseGroup, error) {
	if rg, ok := f.releaseGroups[mbid]; ok {
		return &rg, nil
	}
	return nil, ports.ErrNotFound
}

func (f *fakeMusicBrainz) ListReleaseGroupsForArtist(_ context.Context, _ string) ([]ports.ReleaseGroup, error) {
	return nil, nil
}

func (f *fakeMusicBrainz) SearchReleaseGroups(_ context.Context, artistName, albumName string) ([]ports.ReleaseGroup, error) {
	f.searchReleaseGroupCalls++
	return f.releaseGroupsByQuery[artistName+"|"+albumName], nil
}

func (f *fakeMusicBrainz) LookupRelease(_ context.Context, mbid string) (*ports.Release, error) {
	f.lookupReleaseCalls++
	if r, ok := f.releases[mbid]; ok {
		return &r, nil
	}
	return nil, ports.ErrNotFound
}

func (f *fakeMusicBrainz) ListReleasesForReleaseGroup(_ context.Context, rgMBID string) ([]ports.Release, error) {
	return f.releasesByRG[rgMBID], nil
}

func (f *fakeMusicBrainz) SearchReleaseByBarcode(_ context.Context, barcode string) ([]ports.Release, error) {
	return f.releasesByBarcode[barcode], nil
}

func (f *fakeMusicBrainz) LookupRecordingByISRC(_ context.Context, isrc string) ([]ports.Recording, error) {
	if recs, ok := f.recordingsByISRC[isrc]; ok {
		return recs, nil
	}
	return nil, ports.ErrNotFound
}

// fakeAcoustID is a hand-rolled ports.AcoustIDClient double. matchesByPath
// maps a file path directly to the AcoustID matches it should resolve to —
// Fingerprint's returned value round-trips the path itself so Lookup can
// find it again without a real fpcalc/API call.
type fakeAcoustID struct {
	matchesByPath map[string][]ports.AcoustIDMatch

	fingerprintCalls int
	lookupCalls      int
}

var _ ports.AcoustIDClient = (*fakeAcoustID)(nil)

func (f *fakeAcoustID) Fingerprint(_ context.Context, path string) (string, float64, error) {
	f.fingerprintCalls++
	return "fp:" + path, 200.0, nil
}

func (f *fakeAcoustID) Lookup(_ context.Context, fingerprint string, _ float64) ([]ports.AcoustIDMatch, error) {
	f.lookupCalls++
	path := strings.TrimPrefix(fingerprint, "fp:")
	if m, ok := f.matchesByPath[path]; ok && len(m) > 0 {
		return m, nil
	}
	return nil, ports.ErrNotFound
}

// fakeFilenameParser is a hand-rolled ports.FilenameParser double.
type fakeFilenameParser struct {
	artist, album string
	ok            bool
	calls         int
}

var _ ports.FilenameParser = (*fakeFilenameParser)(nil)

func (f *fakeFilenameParser) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

func (f *fakeFilenameParser) Parse(_ context.Context, _, _ string) (artist, album string, ok bool) {
	f.calls++
	return f.artist, f.album, f.ok
}

func noGuessFilenameParser() *fakeFilenameParser { return &fakeFilenameParser{} }

// assertNeverScoredAndCarriesReleaseGroup enforces two checklist items
// across every candidate a test produces: M7 must never set Score (the
// hard M7/M8 boundary), and every candidate must carry
// Metadata["release_group_mbid"] for M8's ambiguity clustering.
func assertNeverScoredAndCarriesReleaseGroup(t *testing.T, candidates []domain.MatchCandidate) {
	t.Helper()
	for _, c := range candidates {
		if c.Score != 0 {
			t.Errorf("candidate %q has Score = %v, want unset (M7 never sets Score)", c.ExternalRef, c.Score)
		}
		rgMBID, ok := c.Metadata["release_group_mbid"]
		if !ok || rgMBID == "" {
			t.Errorf("candidate %q Metadata[release_group_mbid] = %v, want a non-empty value", c.ExternalRef, rgMBID)
		}
	}
}

func TestIdentifier_ContentTypes(t *testing.T) {
	id := music.NewIdentifier(newFakeMusicBrainz(), &fakeAcoustID{}, noGuessFilenameParser())
	got := id.ContentTypes()
	if len(got) != 1 || got[0] != domain.ContentTypeMusic {
		t.Fatalf("ContentTypes() = %v, want [%v]", got, domain.ContentTypeMusic)
	}
}

func TestIdentifier_NothingFoundReturnsEmpty(t *testing.T) {
	mb := newFakeMusicBrainz()
	id := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())

	fp := domain.Fingerprint{Tags: map[string]string{}, Metadata: map[string]any{"track_count": 5}}
	got, err := id.Identify(context.Background(), fp, []string{"/music/a.flac"}, "/music/Unknown", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Identify() = %+v, want empty", got)
	}
}

func TestIdentifier_DirectID_AlbumMBIDResolves(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releases["rel-1"] = ports.Release{
		ID:           "rel-1",
		Title:        "Hi Infidelity",
		Media:        []ports.Medium{{Position: 1, TrackCount: 10}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-1", Title: "Hi Infidelity"},
	}
	id := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())

	fp := domain.Fingerprint{
		Tags:     map[string]string{},
		Metadata: map[string]any{"track_count": 10, "embedded_release_mbids": []string{"rel-1"}},
	}
	got, err := id.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Identify() returned %d candidates, want 1", len(got))
	}
	if got[0].ExternalRef != "rel-1" || got[0].Tier != domain.MatchTierDirectID {
		t.Fatalf("candidate = %+v, want ExternalRef=rel-1 Tier=direct_id", got[0])
	}
	if mb.searchReleaseGroupCalls != 0 {
		t.Errorf("SearchReleaseGroups called %d times, want 0 (direct-ID success must short-circuit step 2)", mb.searchReleaseGroupCalls)
	}
	assertNeverScoredAndCarriesReleaseGroup(t, got)
}

func TestIdentifier_DirectID_ReleaseGroupMBIDFallback(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releaseGroups["rg-1"] = ports.ReleaseGroup{ID: "rg-1", Title: "Hi Infidelity"}
	mb.releasesByRG["rg-1"] = []ports.Release{{ID: "rel-1", Media: []ports.Medium{{Position: 1, TrackCount: 10}}}}
	mb.releases["rel-1"] = ports.Release{
		ID:           "rel-1",
		Title:        "Hi Infidelity",
		Media:        []ports.Medium{{Position: 1, TrackCount: 10}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-1", Title: "Hi Infidelity"},
	}
	id := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())

	fp := domain.Fingerprint{
		Tags:     map[string]string{},
		Metadata: map[string]any{"track_count": 10, "embedded_releasegroup_mbids": []string{"rg-1"}},
	}
	got, err := id.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(got) != 1 || got[0].ExternalRef != "rel-1" || got[0].Tier != domain.MatchTierDirectID {
		t.Fatalf("Identify() = %+v, want one direct_id candidate ExternalRef=rel-1", got)
	}
}

func TestIdentifier_DirectID_ConflictingEmbeddedIDsSkipStep1(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["Some Artist|Some Album"] = []ports.ReleaseGroup{{ID: "rg-fallback", Title: "Some Album"}}
	mb.releasesByRG["rg-fallback"] = []ports.Release{{ID: "rel-fallback"}}
	mb.releases["rel-fallback"] = ports.Release{
		ID:           "rel-fallback",
		Title:        "Some Album",
		Media:        []ports.Medium{{Position: 1, TrackCount: 10}},
		ArtistCredit: []ports.ArtistCredit{{Name: "Some Artist"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-fallback", Title: "Some Album"},
	}
	id := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())

	fp := domain.Fingerprint{
		Tags: map[string]string{"ALBUMARTIST": "Some Artist", "ALBUM": "Some Album"},
		Metadata: map[string]any{
			"track_count":            10,
			"embedded_release_mbids": []string{"conflict-a", "conflict-b"},
		},
	}
	got, err := id.Identify(context.Background(), fp, nil, "/music/Some Artist/Some Album", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if mb.lookupReleaseCalls == 0 {
		t.Fatal("expected a step-2 LookupRelease call, but none happened")
	}
	for _, c := range got {
		if c.Tier == domain.MatchTierDirectID {
			t.Errorf("candidate %+v has Tier=direct_id, want a conflicting embedded-ID set to skip step 1 entirely", c)
		}
	}
	if len(got) != 1 || got[0].ExternalRef != "rel-fallback" {
		t.Fatalf("Identify() = %+v, want one candidate ExternalRef=rel-fallback found via step 2", got)
	}
}

func TestIdentifier_DirectID_SanityCheckFailureFallsThrough(t *testing.T) {
	mb := newFakeMusicBrainz()
	// The embedded MBID resolves, but to a release whose track count is
	// wildly different from the group's — the sanity check must reject it
	// and fall through to step 2, not accept it and not return empty.
	mb.releases["bad-rel"] = ports.Release{
		ID:    "bad-rel",
		Title: "Wrong Album",
		Media: []ports.Medium{{Position: 1, TrackCount: 2}},
	}
	mb.releaseGroupsByQuery["Some Artist|Some Album"] = []ports.ReleaseGroup{{ID: "rg-correct", Title: "Some Album"}}
	mb.releasesByRG["rg-correct"] = []ports.Release{{ID: "rel-correct"}}
	mb.releases["rel-correct"] = ports.Release{
		ID:           "rel-correct",
		Title:        "Some Album",
		Media:        []ports.Medium{{Position: 1, TrackCount: 10}},
		ArtistCredit: []ports.ArtistCredit{{Name: "Some Artist"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-correct", Title: "Some Album"},
	}
	id := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())

	fp := domain.Fingerprint{
		Tags: map[string]string{"ALBUMARTIST": "Some Artist", "ALBUM": "Some Album"},
		Metadata: map[string]any{
			"track_count":            10,
			"embedded_release_mbids": []string{"bad-rel"},
		},
	}
	got, err := id.Identify(context.Background(), fp, nil, "/music/Some Artist/Some Album", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("Identify() returned no candidates, want the sanity-check failure to fall through to step 2, not return empty")
	}
	for _, c := range got {
		if c.ExternalRef == "bad-rel" {
			t.Errorf("candidate list contains the sanity-check-failed release %q, want it rejected", c.ExternalRef)
		}
		if c.Tier == domain.MatchTierDirectID {
			t.Errorf("candidate %+v has Tier=direct_id, want the sanity-check failure to prevent a direct_id accept", c)
		}
	}
	found := false
	for _, c := range got {
		if c.ExternalRef == "rel-correct" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Identify() = %+v, want it to contain the step-2-discovered rel-correct candidate", got)
	}
}

func TestIdentifier_AcoustID_SkippedWhenBarcodeMatchFound(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releasesByBarcode["012345"] = []ports.Release{{ID: "rel-barcode"}}
	mb.releases["rel-barcode"] = ports.Release{
		ID:           "rel-barcode",
		Title:        "Some Album",
		Barcode:      "012345",
		Media:        []ports.Medium{{Position: 1, TrackCount: 10}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-x", Title: "Some Album"},
	}
	acoustID := &fakeAcoustID{}
	id := music.NewIdentifier(mb, acoustID, noGuessFilenameParser())

	fp := domain.Fingerprint{
		Tags:     map[string]string{"BARCODE": "012345"},
		Metadata: map[string]any{"track_count": 10},
	}
	_, err := id.Identify(context.Background(), fp, []string{"/music/a.flac", "/music/b.flac"}, "/music/Some Album", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if acoustID.fingerprintCalls != 0 {
		t.Errorf("AcoustID.Fingerprint called %d times, want 0 (a barcode match was found)", acoustID.fingerprintCalls)
	}
}

func TestIdentifier_AcoustID_RunsWhenNoUniqueIDMatchFound(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["Some Artist|Some Album"] = []ports.ReleaseGroup{{ID: "rg-x", Title: "Some Album"}}
	mb.releasesByRG["rg-x"] = []ports.Release{{ID: "rel-x"}}
	mb.releases["rel-x"] = ports.Release{
		ID:           "rel-x",
		Title:        "Some Album",
		Media:        []ports.Medium{{Position: 1, TrackCount: 2}},
		ArtistCredit: []ports.ArtistCredit{{Name: "Some Artist"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-x", Title: "Some Album"},
	}
	acoustID := &fakeAcoustID{matchesByPath: map[string][]ports.AcoustIDMatch{}}
	id := music.NewIdentifier(mb, acoustID, noGuessFilenameParser())

	fp := domain.Fingerprint{
		Tags:     map[string]string{"ALBUMARTIST": "Some Artist", "ALBUM": "Some Album"},
		Metadata: map[string]any{"track_count": 2},
	}
	paths := []string{"/music/a.flac", "/music/b.flac"}
	_, err := id.Identify(context.Background(), fp, paths, "/music/Some Album", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if acoustID.fingerprintCalls != len(paths) {
		t.Errorf("AcoustID.Fingerprint called %d times, want %d (one per file, no unique-ID match found)", acoustID.fingerprintCalls, len(paths))
	}
}

// TestIdentifier_WorkedExample1_HiInfidelity exercises
// docs/technical/music-identification.md's single-disc worked example: no
// barcode/ISRC/embedded MB ID, fuzzy tag search returns one release group
// with two nearly-identical editions, both landing at Tier=fuzzy with
// strong signal agreement.
func TestIdentifier_WorkedExample1_HiInfidelity(t *testing.T) {
	titles := []string{"T1", "T2", "T3", "T4", "T5", "T6", "T7", "T8", "T9", "T10"}
	durations := []float64{180, 190, 200, 210, 220, 230, 240, 250, 260, 270}
	// One track runs a few seconds long on this rip — still within
	// duration tolerance is false for this one position.
	riderDurations := append([]float64(nil), durations...)
	riderDurations[4] += 15

	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["REO Speedwagon|Hi Infidelity"] = []ports.ReleaseGroup{{ID: "rg-hi-infidelity", Title: "Hi Infidelity"}}
	mb.releasesByRG["rg-hi-infidelity"] = []ports.Release{{ID: "rel-1980"}, {ID: "rel-2004"}}

	makeTracks := func() []ports.Track {
		tracks := make([]ports.Track, len(titles))
		for i, title := range titles {
			tracks[i] = ports.Track{Position: i + 1, Title: title, Length: int(durations[i] * 1000)}
		}
		return tracks
	}
	for _, id := range []string{"rel-1980", "rel-2004"} {
		mb.releases[id] = ports.Release{
			ID:           id,
			Title:        "Hi Infidelity",
			Media:        []ports.Medium{{Position: 1, TrackCount: len(titles), Tracks: makeTracks()}},
			ArtistCredit: []ports.ArtistCredit{{Name: "REO Speedwagon"}},
			ReleaseGroup: &ports.ReleaseGroup{ID: "rg-hi-infidelity", Title: "Hi Infidelity"},
		}
	}

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())

	fp := domain.Fingerprint{
		Tags: map[string]string{"ALBUMARTIST": "REO Speedwagon", "ALBUM": "Hi Infidelity"},
		Metadata: map[string]any{
			"track_count":     10,
			"track_titles":    titles,
			"track_durations": riderDurations,
			"track_isrcs":     make([]string, 10),
		},
	}
	got, err := identifier.Identify(context.Background(), fp, nil, "/music/REO Speedwagon/Hi Infidelity (1980)", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Identify() returned %d candidates, want 2 (both editions)", len(got))
	}
	assertNeverScoredAndCarriesReleaseGroup(t, got)

	for _, c := range got {
		if c.Tier != domain.MatchTierFuzzy {
			t.Errorf("candidate %q Tier = %q, want fuzzy", c.ExternalRef, c.Tier)
		}
		if c.Metadata["release_group_mbid"] != "rg-hi-infidelity" {
			t.Errorf("candidate %q release_group_mbid = %v, want rg-hi-infidelity", c.ExternalRef, c.Metadata["release_group_mbid"])
		}
		if v := c.Signals["name_fuzzy_score"]; v < 0.99 {
			t.Errorf("candidate %q name_fuzzy_score = %v, want ~1.0 (exact tag match)", c.ExternalRef, v)
		}
		if v := c.Signals["track_count_match"]; v != 1.0 {
			t.Errorf("candidate %q track_count_match = %v, want 1.0 (10/10)", c.ExternalRef, v)
		}
		if v := c.Signals["title_set_overlap"]; v != 1.0 {
			t.Errorf("candidate %q title_set_overlap = %v, want 1.0 (10/10)", c.ExternalRef, v)
		}
		if v := c.Signals["duration_match"]; v != 0.9 {
			t.Errorf("candidate %q duration_match = %v, want 0.9 (9/10, one track outside tolerance)", c.ExternalRef, v)
		}
		if _, ok := c.Signals["barcode_match"]; ok {
			t.Errorf("candidate %q has barcode_match set, want absent (no barcode tag)", c.ExternalRef)
		}
		if _, ok := c.Signals["isrc_consensus_fraction"]; ok {
			t.Errorf("candidate %q has isrc_consensus_fraction set, want absent (no ISRC tags)", c.ExternalRef)
		}
	}
}

// TestIdentifier_WorkedExample2_StevieNicksBoxSet exercises
// docs/technical/music-identification.md's 3-disc box-set worked example:
// fuzzy name search returns two release groups (the real box set and an
// unrelated single-disc compilation whose title also fuzzy-matches);
// track/medium counts are what actually separate them, not the name
// signal alone. AcoustID structurally runs here too (no barcode/ISRC
// tags) per docs/technical/pipeline-music-identifier.md's gate being
// purely "no unique-ID match found" — this test does not assert AcoustID
// was skipped; the worked-example doc's narrative was corrected to match
// (AcoustID runs but isn't what decides the outcome here).
func TestIdentifier_WorkedExample2_StevieNicksBoxSet(t *testing.T) {
	groupTitles := make([]string, 32)
	groupDurations := make([]float64, 32)
	for i := range groupTitles {
		groupTitles[i] = "Track " + string(rune('A'+i%26))
		groupDurations[i] = 200
	}
	// Two tracks differ only in minor text formatting from the real
	// candidate's titles — still a probable match, not a miss.
	realTitles := append([]string(nil), groupTitles...)
	realTitles[30] = groupTitles[30] + " (Live)"
	realTitles[31] = groupTitles[31] + " (Live)"

	mb := newFakeMusicBrainz()
	mb.releaseGroupsByQuery["Stevie Nicks|Enhanced"] = []ports.ReleaseGroup{
		{ID: "rg-real", Title: "Enhanced"},
		{ID: "rg-fake", Title: "Enhanced"},
	}
	mb.releasesByRG["rg-real"] = []ports.Release{{ID: "rel-real"}}
	mb.releasesByRG["rg-fake"] = []ports.Release{{ID: "rel-fake"}}

	realTracks := make([]ports.Track, 32)
	for i, title := range realTitles {
		realTracks[i] = ports.Track{Position: i + 1, Title: title, Length: int(groupDurations[i] * 1000)}
	}
	mb.releases["rel-real"] = ports.Release{
		ID:    "rel-real",
		Title: "Enhanced",
		Media: []ports.Medium{
			{Position: 1, TrackCount: 12, Tracks: realTracks[0:12]},
			{Position: 2, TrackCount: 12, Tracks: realTracks[12:24]},
			{Position: 3, TrackCount: 8, Tracks: realTracks[24:32]},
		},
		ArtistCredit: []ports.ArtistCredit{{Name: "Stevie Nicks"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-real", Title: "Enhanced"},
	}
	fakeTracks := make([]ports.Track, 11)
	for i := range fakeTracks {
		fakeTracks[i] = ports.Track{Position: i + 1, Title: "Unrelated Track", Length: 150000}
	}
	mb.releases["rel-fake"] = ports.Release{
		ID:           "rel-fake",
		Title:        "Enhanced",
		Media:        []ports.Medium{{Position: 1, TrackCount: 11, Tracks: fakeTracks}},
		ArtistCredit: []ports.ArtistCredit{{Name: "Various Artists"}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-fake", Title: "Enhanced"},
	}

	identifier := music.NewIdentifier(mb, &fakeAcoustID{}, noGuessFilenameParser())

	fp := domain.Fingerprint{
		Tags: map[string]string{"ALBUMARTIST": "Stevie Nicks", "ALBUM": "Enhanced"},
		Metadata: map[string]any{
			"track_count":     32,
			"disc_count":      3,
			"track_titles":    groupTitles,
			"track_durations": groupDurations,
			"track_isrcs":     make([]string, 32),
		},
	}
	paths := make([]string, 32)
	for i := range paths {
		paths[i] = "/music/Stevie Nicks/Enhanced/f.flac"
	}
	got, err := identifier.Identify(context.Background(), fp, paths, "/music/Stevie Nicks/Enhanced [Box Set]", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	assertNeverScoredAndCarriesReleaseGroup(t, got)

	var real, fake *domain.MatchCandidate
	for i := range got {
		switch got[i].ExternalRef {
		case "rel-real":
			real = &got[i]
		case "rel-fake":
			fake = &got[i]
		}
	}
	if real == nil || fake == nil {
		t.Fatalf("Identify() = %+v, want both rel-real and rel-fake as candidates", got)
	}

	if real.Signals["track_count_match"] != 1.0 {
		t.Errorf("real candidate track_count_match = %v, want 1.0 (32/32)", real.Signals["track_count_match"])
	}
	if fake.Signals["track_count_match"] >= 0.5 {
		t.Errorf("fake candidate track_count_match = %v, want a large gap from the real candidate's 1.0", fake.Signals["track_count_match"])
	}
	if v := real.Signals["title_set_overlap"]; v <= 0.9 || v >= 1.0 {
		t.Errorf("real candidate title_set_overlap = %v, want just under 1.0 (30 exact + 2 partial-credit near-misses out of 32)", v)
	}
	if real.Tier != domain.MatchTierFuzzy {
		t.Errorf("real candidate Tier = %q, want fuzzy", real.Tier)
	}
}

func TestIdentifier_ISRCConsensus_UniqueIDTier(t *testing.T) {
	mb := newFakeMusicBrainz()
	for _, isrc := range []string{"ISRC1", "ISRC2", "ISRC3"} {
		mb.recordingsByISRC[isrc] = []ports.Recording{{ID: "rec-" + isrc, Releases: []ports.Release{{ID: "rel-isrc"}}}}
	}
	mb.releases["rel-isrc"] = ports.Release{
		ID:    "rel-isrc",
		Title: "ISRC Album",
		Media: []ports.Medium{{Position: 1, TrackCount: 3, Tracks: []ports.Track{
			{Position: 1, Title: "T1", Recording: &ports.Recording{ISRCs: []string{"ISRC1"}}},
			{Position: 2, Title: "T2", Recording: &ports.Recording{ISRCs: []string{"ISRC2"}}},
			{Position: 3, Title: "T3", Recording: &ports.Recording{ISRCs: []string{"ISRC3"}}},
		}}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-isrc", Title: "ISRC Album"},
	}
	acoustID := &fakeAcoustID{}
	id := music.NewIdentifier(mb, acoustID, noGuessFilenameParser())

	fp := domain.Fingerprint{
		Tags:     map[string]string{},
		Metadata: map[string]any{"track_count": 3, "track_isrcs": []string{"ISRC1", "ISRC2", "ISRC3"}},
	}
	got, err := id.Identify(context.Background(), fp, []string{"/music/a.flac"}, "/music/ISRC Album", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Identify() returned %d candidates, want 1", len(got))
	}
	if v := got[0].Signals["isrc_consensus_fraction"]; v != 1.0 {
		t.Errorf("isrc_consensus_fraction = %v, want 1.0 (3/3)", v)
	}
	if got[0].Tier != domain.MatchTierUniqueID {
		t.Errorf("Tier = %q, want unique_id", got[0].Tier)
	}
	if acoustID.fingerprintCalls != 0 {
		t.Errorf("AcoustID.Fingerprint called %d times, want 0 (ISRC consensus is a unique-ID match)", acoustID.fingerprintCalls)
	}
	assertNeverScoredAndCarriesReleaseGroup(t, got)
}

func TestIdentifier_AcoustID_SoleEvidenceTier(t *testing.T) {
	mb := newFakeMusicBrainz()
	mb.releasesByRG["rg-acoustic"] = []ports.Release{{ID: "rel-acoustic"}}
	mb.releases["rel-acoustic"] = ports.Release{
		ID:           "rel-acoustic",
		Title:        "Acoustic Album",
		Media:        []ports.Medium{{Position: 1, TrackCount: 2}},
		ReleaseGroup: &ports.ReleaseGroup{ID: "rg-acoustic", Title: "Acoustic Album"},
	}
	paths := []string{"/music/a.flac", "/music/b.flac"}
	matches := []ports.AcoustIDMatch{{
		AcoustID: "aid-1",
		Recordings: []ports.AcoustIDRecording{{
			MBID:          "rec-1",
			ReleaseGroups: []ports.AcoustIDReleaseGroup{{MBID: "rg-acoustic", Title: "Acoustic Album"}},
		}},
	}}
	acoustID := &fakeAcoustID{matchesByPath: map[string][]ports.AcoustIDMatch{
		paths[0]: matches,
		paths[1]: matches,
	}}
	id := music.NewIdentifier(mb, acoustID, noGuessFilenameParser())

	fp := domain.Fingerprint{Tags: map[string]string{}, Metadata: map[string]any{}}
	got, err := id.Identify(context.Background(), fp, paths, "/music/Unknown", "/music")
	if err != nil {
		t.Fatalf("Identify returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Identify() returned %d candidates, want 1", len(got))
	}
	if acoustID.fingerprintCalls != len(paths) {
		t.Errorf("AcoustID.Fingerprint called %d times, want %d", acoustID.fingerprintCalls, len(paths))
	}
	if v := got[0].Signals["acoustic_agreement"]; v != 1.0 {
		t.Errorf("acoustic_agreement = %v, want 1.0 (both files agree)", v)
	}
	if got[0].Tier != domain.MatchTierAcoustic {
		t.Errorf("Tier = %q, want acoustic (AcoustID is the sole evidence)", got[0].Tier)
	}
	assertNeverScoredAndCarriesReleaseGroup(t, got)
}
