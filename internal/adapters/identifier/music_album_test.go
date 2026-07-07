package identifier_test

import (
	"context"
	"purser/internal/adapters/identifier"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// ── fakes ──────────────────────────────────────────────────────────────────────

type stubMusicReleaseRepo struct {
	byMBID map[string]*domain.MusicRelease
}

func (r *stubMusicReleaseRepo) Get(context.Context, string) (*domain.MusicRelease, error) {
	return nil, errs.ErrNotFound
}

func (r *stubMusicReleaseRepo) GetByMBID(_ context.Context, mbid string) (*domain.MusicRelease, error) {
	if rel, ok := r.byMBID[mbid]; ok {
		return rel, nil
	}
	return nil, errs.ErrNotFound
}

func (r *stubMusicReleaseRepo) GetByBarcode(context.Context, string) (*domain.MusicRelease, error) {
	return nil, errs.ErrNotFound
}

func (r *stubMusicReleaseRepo) ListByGroup(context.Context, string) ([]*domain.MusicRelease, error) {
	return nil, nil
}

func (r *stubMusicReleaseRepo) ListByEntry(context.Context, string) ([]*domain.MusicRelease, error) {
	return nil, nil
}

func (r *stubMusicReleaseRepo) ListTracksByRelease(context.Context, string) ([]*domain.Item, error) {
	return nil, nil
}
func (r *stubMusicReleaseRepo) Save(context.Context, *domain.MusicRelease) error { return nil }
func (r *stubMusicReleaseRepo) Delete(context.Context, string) error             { return nil }

type recordingImporter struct {
	calls []domain.MusicReleaseCandidate
	err   error
}

func (i *recordingImporter) ImportRelease(_ context.Context, c domain.MusicReleaseCandidate, _ ports.ScannedFileGroup) error {
	i.calls = append(i.calls, c)
	return i.err
}

// stubMusicSource is a configurable MetadataSource implementing the capability
// sub-interfaces the album identifier type-asserts.
type stubMusicSource struct {
	barcodeRel  *ports.ExternalMusicRelease
	barcodeErr  error
	isrcToRG    map[string]string
	rgReleases  map[string][]*ports.ExternalMusicRelease
	artists     []*domain.ExternalStudio
	discography map[string][]*domain.ExternalGroup
}

func (s *stubMusicSource) Name() string { return "stub" }
func (s *stubMusicSource) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}
func (s *stubMusicSource) ImagePriority() int { return 0 }

func (s *stubMusicSource) GetReleaseByBarcode(_ context.Context, _ string) (*ports.ExternalMusicRelease, error) {
	if s.barcodeErr != nil {
		return nil, s.barcodeErr
	}
	if s.barcodeRel == nil {
		return nil, ports.ErrNotFound
	}
	return s.barcodeRel, nil
}

func (s *stubMusicSource) LookupISRC(_ context.Context, isrc string) (string, error) {
	return s.isrcToRG[isrc], nil
}

func (s *stubMusicSource) FetchReleaseGroupReleases(_ context.Context, rgMBID string) ([]*ports.ExternalMusicRelease, error) {
	return s.rgReleases[rgMBID], nil
}

func (s *stubMusicSource) SearchStudios(_ context.Context, _ string, _ int) ([]*domain.ExternalStudio, error) {
	return s.artists, nil
}

func (s *stubMusicSource) FetchEntryContent(_ context.Context, _ domain.ContentType, extID string, _, _ int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	groups := s.discography[extID]
	return groups, nil, len(groups), nil
}

// ── helpers ────────────────────────────────────────────────────────────────────

// taggedFiles builds a group of scanned files sharing album-level tags, each
// with its own track number, title, ISRC, and duration.
func taggedFiles(root string, album map[string]string, tracks []map[string]string) ports.ScannedFileGroup {
	fs := make([]domain.ScannedFile, len(tracks))
	for i, tr := range tracks {
		tags := map[string]string{}
		for k, v := range album {
			tags[k] = v
		}
		for k, v := range tr {
			tags[k] = v
		}
		fs[i] = domain.ScannedFile{
			Path:        root + "/" + tags["track_number"] + ".flac",
			ContentType: domain.ContentTypeMusic,
			Fingerprint: &domain.Fingerprint{EmbeddedTags: tags},
		}
	}
	return ports.ScannedFileGroup{RootPath: root, Files: fs}
}

func newAlbumID(queue ports.MusicScanGroupRepository, releases ports.MusicReleaseRepository, src *stubMusicSource, imp ports.AlbumImporter) ports.GroupIdentifier {
	var sources []ports.MetadataSource
	if src != nil {
		sources = []ports.MetadataSource{src}
	}
	return identifier.NewMusicAlbumIdentifier(queue, releases, sources, imp, 0.85)
}

// ── tests ──────────────────────────────────────────────────────────────────────

func TestMusicAlbumIdentifier_ReScanShortcut_SkipsKnownImport(t *testing.T) {
	queue := &stubMusicScanGroupRepo{}
	releases := &stubMusicReleaseRepo{byMBID: map[string]*domain.MusicRelease{
		"REL-1": {ID: "r1", Status: domain.ReleaseStatusImported},
	}}
	imp := &recordingImporter{}
	id := newAlbumID(queue, releases, &stubMusicSource{}, imp)

	group := taggedFiles("/music/Hi Infidelity",
		map[string]string{"album_artist": "REO", "album": "Hi Infidelity", "musicbrainz_album_id": "REL-1", "track_total": "1"},
		[]map[string]string{{"track_number": "1", "title": "Track"}})

	if err := id.Identify(context.Background(), group); err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if len(imp.calls) != 0 {
		t.Errorf("importer must not be called on re-scan shortcut, got %d calls", len(imp.calls))
	}
	if len(queue.entries) != 0 {
		t.Errorf("queue must be untouched on re-scan shortcut, got %d entries", len(queue.entries))
	}
}

func TestMusicAlbumIdentifier_ReScanShortcut_ProceedsIfNotImported(t *testing.T) {
	queue := &stubMusicScanGroupRepo{}
	releases := &stubMusicReleaseRepo{byMBID: map[string]*domain.MusicRelease{
		"REL-1": {ID: "r1", Status: domain.ReleaseStatusStub},
	}}
	imp := &recordingImporter{}
	id := newAlbumID(queue, releases, &stubMusicSource{}, imp)

	group := taggedFiles("/music/Hi Infidelity",
		map[string]string{"album_artist": "REO", "album": "Hi Infidelity", "musicbrainz_album_id": "REL-1", "track_total": "1"},
		[]map[string]string{{"track_number": "1", "title": "Track"}})

	if err := id.Identify(context.Background(), group); err != nil {
		t.Fatalf("Identify: %v", err)
	}
	// Stub (not imported) → pipeline proceeds and queues a pending entry.
	if len(queue.entries) != 1 {
		t.Fatalf("expected pipeline to proceed and queue 1 entry, got %d", len(queue.entries))
	}
	if queue.entries[0].Status != domain.UnmatchedPending {
		t.Errorf("entry status: got %v, want pending", queue.entries[0].Status)
	}
}

func TestMusicAlbumIdentifier_BarcodeHit_AutoImports(t *testing.T) {
	// Pre-seed the pending entry the way MusicGroupQueueWriter would.
	seeded := &domain.MusicScanGroup{ID: "seed", FolderPath: "/music/Hi Infidelity", Status: domain.UnmatchedPending}
	queue := &stubMusicScanGroupRepo{entries: []*domain.MusicScanGroup{seeded}}
	releases := &stubMusicReleaseRepo{}
	imp := &recordingImporter{}
	src := &stubMusicSource{
		barcodeRel: &ports.ExternalMusicRelease{MBID: "rel-1", Title: "Hi Infidelity", Barcode: "074646161425", TrackCount: 10},
	}
	id := newAlbumID(queue, releases, src, imp)

	tracks := make([]map[string]string, 10)
	for i := range tracks {
		tracks[i] = map[string]string{"track_number": string(rune('0' + i)), "title": "T"}
	}
	group := taggedFiles("/music/Hi Infidelity",
		map[string]string{"album_artist": "REO", "album": "Hi Infidelity (2024 Remaster)", "barcode": "0074646161425", "track_total": "10"},
		tracks)

	if err := id.Identify(context.Background(), group); err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if len(imp.calls) != 1 {
		t.Fatalf("expected exactly 1 import call, got %d", len(imp.calls))
	}
	if imp.calls[0].ReleaseMBID != "rel-1" {
		t.Errorf("imported release MBID: got %q, want rel-1", imp.calls[0].ReleaseMBID)
	}
	if imp.calls[0].Signals.Barcode != 1.0 {
		t.Errorf("barcode signal: got %v, want 1.0", imp.calls[0].Signals.Barcode)
	}
	if imp.calls[0].OverallConfidence != 1.0 {
		t.Errorf("overall confidence: got %v, want 1.0", imp.calls[0].OverallConfidence)
	}
	if len(queue.entries) != 1 || queue.entries[0].Status != domain.UnmatchedMatched {
		t.Fatalf("seeded entry must be flipped to matched, got %+v", queue.entries)
	}
	if queue.entries[0].ID != "seed" {
		t.Errorf("must update seeded entry, not create a new one; got id %q", queue.entries[0].ID)
	}
}

func TestMusicAlbumIdentifier_BelowThreshold_SavesQueue(t *testing.T) {
	queue := &stubMusicScanGroupRepo{}
	releases := &stubMusicReleaseRepo{}
	imp := &recordingImporter{}
	src := &stubMusicSource{
		artists:     []*domain.ExternalStudio{{ExternalID: "artist-1", Name: "Stevie Nicks"}},
		discography: map[string][]*domain.ExternalGroup{"artist-1": {{ExternalID: "rg-1", Title: "The Enchanted Works of Stevie Nicks"}}},
		rgReleases: map[string][]*ports.ExternalMusicRelease{
			"rg-1": {{MBID: "rel-a", Title: "The Enchanted Works of Stevie Nicks", TrackCount: 30}},
		},
	}
	id := newAlbumID(queue, releases, src, imp)

	group := taggedFiles("/music/Enchanted",
		map[string]string{"album_artist": "Stevie Nicks", "album": "The Enchanted Works of Stevie Nicks", "track_total": "30"},
		[]map[string]string{{"track_number": "1", "title": "Enchanted"}})

	if err := id.Identify(context.Background(), group); err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if len(imp.calls) != 0 {
		t.Errorf("below threshold must not import, got %d calls", len(imp.calls))
	}
	if len(queue.entries) != 1 {
		t.Fatalf("expected 1 queued entry, got %d", len(queue.entries))
	}
	e := queue.entries[0]
	if e.Status != domain.UnmatchedPending {
		t.Errorf("status: got %v, want pending", e.Status)
	}
	if len(e.Candidates) == 0 {
		t.Fatal("expected at least one candidate saved")
	}
	if e.Candidates[0].OverallConfidence >= 0.85 {
		t.Errorf("candidate confidence %v should be below threshold", e.Candidates[0].OverallConfidence)
	}
}

func TestMusicAlbumIdentifier_CandidatesRankedByConfidence(t *testing.T) {
	queue := &stubMusicScanGroupRepo{}
	releases := &stubMusicReleaseRepo{}
	imp := &recordingImporter{}
	// Two editions of the same fuzzy-matched RG: one matches the track count,
	// the other does not, so the matching edition must rank first.
	src := &stubMusicSource{
		artists:     []*domain.ExternalStudio{{ExternalID: "artist-1", Name: "Artist"}},
		discography: map[string][]*domain.ExternalGroup{"artist-1": {{ExternalID: "rg-1", Title: "Album"}}},
		rgReleases: map[string][]*ports.ExternalMusicRelease{
			"rg-1": {
				{MBID: "rel-mismatch", Title: "Album", TrackCount: 99},
				{MBID: "rel-match", Title: "Album", TrackCount: 10},
			},
		},
	}
	id := newAlbumID(queue, releases, src, imp)

	group := taggedFiles("/music/Album",
		map[string]string{"album_artist": "Artist", "album": "Album", "track_total": "10"},
		[]map[string]string{{"track_number": "1", "title": "T"}})

	if err := id.Identify(context.Background(), group); err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if len(imp.calls) != 0 {
		t.Fatalf("expected below-threshold queue, not import")
	}
	cands := queue.entries[0].Candidates
	if len(cands) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(cands))
	}
	if cands[0].ReleaseMBID != "rel-match" {
		t.Errorf("top candidate: got %q, want rel-match", cands[0].ReleaseMBID)
	}
	if cands[0].OverallConfidence < cands[1].OverallConfidence {
		t.Errorf("candidates not sorted by confidence: %v < %v", cands[0].OverallConfidence, cands[1].OverallConfidence)
	}
}

func TestMusicAlbumIdentifier_AllSignalBreakdownPresent(t *testing.T) {
	queue := &stubMusicScanGroupRepo{}
	releases := &stubMusicReleaseRepo{}
	imp := &recordingImporter{}
	src := &stubMusicSource{
		artists:     []*domain.ExternalStudio{{ExternalID: "artist-1", Name: "Artist"}},
		discography: map[string][]*domain.ExternalGroup{"artist-1": {{ExternalID: "rg-1", Title: "Album"}}},
		rgReleases: map[string][]*ports.ExternalMusicRelease{
			"rg-1": {{MBID: "rel-a", Title: "Album", TrackCount: 10}},
		},
	}
	id := newAlbumID(queue, releases, src, imp)

	group := taggedFiles("/music/Album",
		map[string]string{"album_artist": "Artist", "album": "Album", "track_total": "10"},
		[]map[string]string{{"track_number": "1", "title": "T"}})

	if err := id.Identify(context.Background(), group); err != nil {
		t.Fatalf("Identify: %v", err)
	}
	sig := queue.entries[0].Candidates[0].Signals

	// The four metadata-derived signals are computed; the three track-detail
	// signals (duration, track-title set, AcoustID) are 0.0 here but present.
	if sig.RGNameFuzzy <= 0 {
		t.Errorf("rgNameFuzzy should be > 0 for an exact name match, got %v", sig.RGNameFuzzy)
	}
	if sig.TrackCount != 1.0 {
		t.Errorf("trackCount should be 1.0 on match, got %v", sig.TrackCount)
	}
	if sig.Barcode != 0 || sig.ISRC != 0 {
		t.Errorf("barcode/isrc should be 0 with no barcode/isrc data, got %v/%v", sig.Barcode, sig.ISRC)
	}
	if sig.Duration != 0 || sig.TrackTitleSet != 0 || sig.AcoustID != 0 {
		t.Errorf("track-detail signals should be 0 in this stage, got d=%v t=%v a=%v", sig.Duration, sig.TrackTitleSet, sig.AcoustID)
	}
}

func TestMusicAlbumIdentifier_ISRCConsensus_ResolvesReleaseGroup(t *testing.T) {
	queue := &stubMusicScanGroupRepo{}
	releases := &stubMusicReleaseRepo{}
	imp := &recordingImporter{}
	src := &stubMusicSource{
		isrcToRG: map[string]string{"USSM10012807": "rg-isrc"},
		rgReleases: map[string][]*ports.ExternalMusicRelease{
			"rg-isrc": {{MBID: "rel-isrc", Title: "Hi Infidelity", TrackCount: 1}},
		},
	}
	id := newAlbumID(queue, releases, src, imp)

	group := taggedFiles("/music/Hi Infidelity",
		map[string]string{"album_artist": "REO", "album": "Hi Infidelity", "track_total": "1"},
		[]map[string]string{{"track_number": "1", "title": "Track", "isrc": "USSM10012807"}})

	if err := id.Identify(context.Background(), group); err != nil {
		t.Fatalf("Identify: %v", err)
	}
	// ISRC consensus (0.95 weight) alone clears the 0.85 threshold → auto-import.
	if len(imp.calls) != 1 {
		t.Fatalf("expected 1 import call from ISRC consensus, got %d", len(imp.calls))
	}
	if imp.calls[0].Signals.ISRC != 1.0 {
		t.Errorf("isrc signal: got %v, want 1.0 (all ISRCs resolved to the RG)", imp.calls[0].Signals.ISRC)
	}
	if imp.calls[0].ReleaseGroupMBID != "rg-isrc" {
		t.Errorf("release group MBID: got %q, want rg-isrc", imp.calls[0].ReleaseGroupMBID)
	}
}

func TestMusicAlbumIdentifier_ReleaseGroupWithoutEditions_StillQueued(t *testing.T) {
	queue := &stubMusicScanGroupRepo{}
	releases := &stubMusicReleaseRepo{}
	imp := &recordingImporter{}
	// Fuzzy match resolves a Release Group, but no editions can be fetched.
	src := &stubMusicSource{
		artists:     []*domain.ExternalStudio{{ExternalID: "artist-1", Name: "Artist"}},
		discography: map[string][]*domain.ExternalGroup{"artist-1": {{ExternalID: "rg-1", Title: "Album"}}},
		rgReleases:  map[string][]*ports.ExternalMusicRelease{}, // no editions
	}
	id := newAlbumID(queue, releases, src, imp)

	group := taggedFiles("/music/Album",
		map[string]string{"album_artist": "Artist", "album": "Album", "track_total": "10"},
		[]map[string]string{{"track_number": "1", "title": "T"}})

	if err := id.Identify(context.Background(), group); err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if len(queue.entries) != 1 || len(queue.entries[0].Candidates) != 1 {
		t.Fatalf("expected 1 queued candidate for the RG, got %+v", queue.entries)
	}
	c := queue.entries[0].Candidates[0]
	if c.ReleaseGroupMBID != "rg-1" {
		t.Errorf("release group MBID: got %q, want rg-1", c.ReleaseGroupMBID)
	}
	if c.ReleaseMBID != "" {
		t.Errorf("release MBID should be empty when no edition was resolved, got %q", c.ReleaseMBID)
	}
}

func TestMusicAlbumIdentifier_ContentTypes(t *testing.T) {
	id := newAlbumID(&stubMusicScanGroupRepo{}, &stubMusicReleaseRepo{}, nil, &recordingImporter{})
	for _, ct := range id.ContentTypes() {
		if ct == domain.ContentTypeMusic {
			return
		}
	}
	t.Fatal("expected ContentTypeMusic in ContentTypes")
}

func TestOverallConfidence_BarcodeShortCircuits(t *testing.T) {
	got := identifier.OverallConfidenceForTest(domain.MusicConfidenceSignals{Barcode: 1.0})
	if got != 1.0 {
		t.Errorf("barcode-only overall: got %v, want 1.0", got)
	}
}

func TestOverallConfidence_WeightedSumCapped(t *testing.T) {
	// rgNameFuzzy 1.0 (0.60) + trackCount 1.0 (0.20) = 0.80
	got := identifier.OverallConfidenceForTest(domain.MusicConfidenceSignals{RGNameFuzzy: 1.0, TrackCount: 1.0})
	if got < 0.79 || got > 0.81 {
		t.Errorf("weighted sum: got %v, want ~0.80", got)
	}
	// Everything maxed must cap at 1.0.
	full := identifier.OverallConfidenceForTest(domain.MusicConfidenceSignals{
		ISRC: 1.0, RGNameFuzzy: 1.0, TrackCount: 1.0, TrackTitleSet: 1.0, Duration: 1.0, AcoustID: 1.0,
	})
	if full != 1.0 {
		t.Errorf("capped overall: got %v, want 1.0", full)
	}
}

func TestNoopAlbumImporter_ReturnsNil(t *testing.T) {
	imp := identifier.NewNoopAlbumImporter()
	err := imp.ImportRelease(context.Background(), domain.MusicReleaseCandidate{}, ports.ScannedFileGroup{RootPath: "/music/x"})
	if err != nil {
		t.Errorf("no-op importer must return nil, got %v", err)
	}
}
