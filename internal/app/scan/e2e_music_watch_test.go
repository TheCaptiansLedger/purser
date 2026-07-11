package scan_test

import (
	"context"
	"os"
	"path/filepath"
	"purser/internal/adapters/fingerprint"
	"purser/internal/adapters/fs"
	"purser/internal/adapters/identifier"
	"purser/internal/app/errs"
	appmusic "purser/internal/app/music"
	"purser/internal/app/scan"
	"purser/internal/domain"
	"purser/internal/ports"
	"sync"
	"testing"
	"time"

	"github.com/go-flac/flacvorbis/v2"
	flac "github.com/go-flac/go-flac/v2"
)

// This file exercises the REAL music album-import pipeline end to end: the real
// fsnotify-backed watcher, the real MusicFolderGrouper, the real
// MusicGroupQueueWriter, the real MusicAlbumIdentifier, and the real
// ReleaseImporter/MusicTagWriter/FileSystem — against real FLAC fixtures copied
// from test-data/music/. Only the actual network boundary (MusicBrainz) is
// stubbed. This is the level nothing else in the test suite verified: every bug
// found in the live debugging session (watcher bypass, duplicate watch events,
// missing album_artist fallback, no real importer) would have failed one of
// these two tests.

const (
	reoArtistMBID  = "artist-mbid-reo-e2e"
	reoRGMBID      = "rg-mbid-hi-infidelity-e2e"
	reoReleaseMBID = "release-mbid-hi-infidelity-e2e"
	reoBarcode     = "0074646161425"
)

// ── real-ish in-memory repos (persist and query correctly, unlike single-field mocks) ──

type e2eMusicReleaseRepo struct {
	mu     sync.Mutex
	byMBID map[string]*domain.MusicRelease
	saved  []*domain.MusicRelease
}

func newE2EMusicReleaseRepo() *e2eMusicReleaseRepo {
	return &e2eMusicReleaseRepo{byMBID: make(map[string]*domain.MusicRelease)}
}

func (r *e2eMusicReleaseRepo) seed(mbid string, rel *domain.MusicRelease) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byMBID[mbid] = rel
}

func (r *e2eMusicReleaseRepo) Get(_ context.Context, id string) (*domain.MusicRelease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rel := range r.byMBID {
		if rel.ID == id {
			cp := *rel
			return &cp, nil
		}
	}
	return nil, errs.ErrNotFound
}

func (r *e2eMusicReleaseRepo) GetByMBID(_ context.Context, mbid string) (*domain.MusicRelease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rel, ok := r.byMBID[mbid]; ok {
		cp := *rel
		return &cp, nil
	}
	return nil, errs.ErrNotFound
}

func (r *e2eMusicReleaseRepo) GetByBarcode(_ context.Context, barcode string) (*domain.MusicRelease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rel := range r.byMBID {
		if rel.Barcode == barcode {
			cp := *rel
			return &cp, nil
		}
	}
	return nil, errs.ErrNotFound
}

func (r *e2eMusicReleaseRepo) ListByGroup(_ context.Context, _ string) ([]*domain.MusicRelease, error) {
	return nil, nil
}

func (r *e2eMusicReleaseRepo) ListByEntry(_ context.Context, _ string) ([]*domain.MusicRelease, error) {
	return nil, nil
}

func (r *e2eMusicReleaseRepo) ListTracksByRelease(_ context.Context, _ string) ([]*domain.Item, error) {
	return nil, nil
}

func (r *e2eMusicReleaseRepo) Save(_ context.Context, rel *domain.MusicRelease) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for mbid, existing := range r.byMBID {
		if existing.ID == rel.ID {
			cp := *rel
			r.byMBID[mbid] = &cp
			r.saved = append(r.saved, &cp)
			return nil
		}
	}
	return nil
}

func (r *e2eMusicReleaseRepo) Delete(_ context.Context, _ string) error { return nil }

func (r *e2eMusicReleaseRepo) savedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.saved)
}

type e2eMusicScanGroupRepo struct {
	mu   sync.Mutex
	byID map[string]*domain.MusicScanGroup
}

func newE2EMusicScanGroupRepo() *e2eMusicScanGroupRepo {
	return &e2eMusicScanGroupRepo{byID: make(map[string]*domain.MusicScanGroup)}
}

func (r *e2eMusicScanGroupRepo) Get(_ context.Context, id string) (*domain.MusicScanGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.byID[id]; ok {
		cp := *g
		return &cp, nil
	}
	return nil, errs.ErrNotFound
}

func (r *e2eMusicScanGroupRepo) List(_ context.Context, status domain.UnmatchedStatus) ([]*domain.MusicScanGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*domain.MusicScanGroup
	for _, g := range r.byID {
		if g.Status == status {
			cp := *g
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *e2eMusicScanGroupRepo) Save(_ context.Context, g *domain.MusicScanGroup) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *g
	r.byID[g.ID] = &cp
	return nil
}

func (r *e2eMusicScanGroupRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byID, id)
	return nil
}

func (r *e2eMusicScanGroupRepo) all() []*domain.MusicScanGroup {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*domain.MusicScanGroup, 0, len(r.byID))
	for _, g := range r.byID {
		cp := *g
		out = append(out, &cp)
	}
	return out
}

// ── stubbed MusicBrainz boundary ──────────────────────────────────────────────

// reoSource stubs the real MusicBrainz adapter for the REO Speedwagon fixture.
// withBarcode controls whether GetReleaseByBarcode resolves — real consumer
// FLAC rips (like the actual test-data fixtures) never carry a barcode tag, so
// tests that use unmodified fixtures must go through the fuzzy-name path.
type reoSource struct {
	withBarcode bool
}

func (s *reoSource) Name() string       { return "mbz" }
func (s *reoSource) ImagePriority() int { return 0 }
func (s *reoSource) ContentTypes() []domain.ContentType {
	return []domain.ContentType{domain.ContentTypeMusic}
}

func (s *reoSource) SearchStudios(_ context.Context, query string, _ int) ([]*domain.ExternalStudio, error) {
	if query != "REO Speedwagon" {
		return nil, nil
	}
	return []*domain.ExternalStudio{{Source: domain.SourceMusicBrainz, ExternalID: reoArtistMBID, Name: "REO Speedwagon"}}, nil
}

func (s *reoSource) FetchEntryContent(_ context.Context, _ domain.ContentType, externalID string, page, _ int) ([]*domain.ExternalGroup, []*domain.ExternalItem, int, error) {
	if externalID != reoArtistMBID || page != 1 {
		return nil, nil, 0, nil
	}
	groups := []*domain.ExternalGroup{{Source: domain.SourceMusicBrainz, ExternalID: reoRGMBID, Title: "Hi Infidelity"}}
	return groups, nil, len(groups), nil
}

func (s *reoSource) FetchReleaseGroupReleases(_ context.Context, rgMBID string) ([]*ports.ExternalMusicRelease, error) {
	if rgMBID != reoRGMBID {
		return nil, nil
	}
	return []*ports.ExternalMusicRelease{{
		MBID: reoReleaseMBID, Title: "Hi Infidelity", TrackCount: 10, Barcode: reoBarcode, IsDefault: true,
	}}, nil
}

func (s *reoSource) GetReleaseByBarcode(_ context.Context, barcode string) (*ports.ExternalMusicRelease, error) {
	if !s.withBarcode || barcode != reoBarcode {
		return nil, ports.ErrNotFound
	}
	return &ports.ExternalMusicRelease{MBID: reoReleaseMBID, Title: "Hi Infidelity", TrackCount: 10, Barcode: reoBarcode}, nil
}

var (
	_ ports.StudioSearchSource        = (*reoSource)(nil)
	_ ports.EntryContentSource        = (*reoSource)(nil)
	_ ports.ReleaseGroupContentSource = (*reoSource)(nil)
	_ ports.BarcodeLookupSource       = (*reoSource)(nil)
)

// ── fixture helpers ───────────────────────────────────────────────────────────

// copyREOFixtures copies every real FLAC in test-data/music/1980 Hi Infidelity
// into dir, returning the number of files copied. Skips the whole test if the
// fixtures are not present (e.g. a checkout without test-data/).
func copyREOFixtures(t *testing.T, dir string) int {
	t.Helper()
	matches, err := filepath.Glob("../../../test-data/music/*/*.flac")
	if err != nil || len(matches) == 0 {
		t.Skip("no test FLAC fixtures found under test-data/music")
	}
	for _, src := range matches {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("read fixture %s: %v", src, err)
		}
		dst := filepath.Join(dir, filepath.Base(src))
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatalf("write copy %s: %v", dst, err)
		}
	}
	return len(matches)
}

// addVorbisComment adds a Vorbis comment tag to a FLAC file on disk, using the
// same libraries the real MusicTagWriter uses. Used to simulate a rip that
// happens to carry a barcode tag, since the real fixtures do not.
func addVorbisComment(t *testing.T, path, key, val string) {
	t.Helper()
	f, err := flac.ParseFile(path)
	if err != nil {
		t.Fatalf("parse flac %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	var cmt *flacvorbis.MetaDataBlockVorbisComment
	idx := -1
	for i, m := range f.Meta {
		if m.Type == flac.VorbisComment {
			if parsed, perr := flacvorbis.ParseFromMetaDataBlock(*m); perr == nil {
				cmt, idx = parsed, i
			}
			break
		}
	}
	if cmt == nil {
		cmt = flacvorbis.New()
	}
	if err := cmt.Add(key, val); err != nil {
		t.Fatalf("add vorbis comment %s=%s: %v", key, val, err)
	}
	block := cmt.Marshal()
	if idx >= 0 {
		f.Meta[idx] = &block
	} else {
		f.Meta = append(f.Meta, &block)
	}
	if err := f.Save(path); err != nil {
		t.Fatalf("save flac %s: %v", path, err)
	}
}

func readVorbisTag(t *testing.T, path, key string) []string {
	t.Helper()
	f, err := flac.ParseFile(path)
	if err != nil {
		t.Fatalf("parse flac %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	for _, m := range f.Meta {
		if m.Type == flac.VorbisComment {
			cmt, err := flacvorbis.ParseFromMetaDataBlock(*m)
			if err != nil {
				continue
			}
			vals, _ := cmt.Get(key)
			return vals
		}
	}
	return nil
}

// newRealMusicPipeline wires the actual production identifier/importer chain —
// the same wiring as cmd/purser/main.go — against a real watcher rooted at dir.
func newRealMusicPipeline(
	releases *e2eMusicReleaseRepo,
	scanGroups *e2eMusicScanGroupRepo,
	items *e2eItemRepo,
	mediaFiles *mockMediaFileRepo,
	src ports.MetadataSource,
) *scan.Service {
	osFS := fs.NewFileSystem()
	tagWriter := fs.NewMusicTagWriter()
	releaseImporter := appmusic.NewReleaseImporter(releases, items, mediaFiles, osFS, tagWriter)

	grouper := identifier.NewMusicFolderGrouper()
	queueWriter := identifier.NewMusicGroupQueueWriter(scanGroups)
	albumIdentifier := identifier.NewMusicAlbumIdentifier(scanGroups, releases, []ports.MetadataSource{src}, releaseImporter, 0.85)

	watcher := fs.NewWatcherNoStabilityCheck(50 * time.Millisecond)
	musicFP := fingerprint.NewMusicFingerprinter()

	return scan.New(
		nil, watcher,
		[]ports.FileFingerprinter{musicFP}, nil,
		[]ports.FileGrouper{grouper},
		[]ports.GroupIdentifier{queueWriter, albumIdentifier},
		items, mediaFiles, newUnmatchedRepo(),
		&mockNotifier{}, 0.85,
		nil, nil, nil, nil, nil,
	)
}

// waitForScanGroup polls scanGroups until at least one entry exists, or fails the test.
func waitForScanGroup(t *testing.T, scanGroups *e2eMusicScanGroupRepo, deadline time.Duration) *domain.MusicScanGroup {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if all := scanGroups.all(); len(all) > 0 {
			return all[0]
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("no music scan group appeared within deadline")
	return nil
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestE2E_RealMusicWatch_UnmodifiedFixtures_QueuesWithoutDuplicates exercises the
// real watcher → grouper → queue-writer → album-identifier chain against the
// real, unmodified REO Speedwagon FLAC fixtures (no barcode/ISRC/album_artist
// tag — exactly what shipped rips look like). It must: reach the album
// identifier at all (watcher bypass), report the true track count (duplicate
// event bug), resolve the artist via the artist-tag fallback (album_artist
// bug), and land in the queue rather than false-auto-importing (confidence
// tops out on name+track-count alone, well under threshold).
func TestE2E_RealMusicWatch_UnmodifiedFixtures_QueuesWithoutDuplicates(t *testing.T) {
	root := t.TempDir()
	staging := t.TempDir()
	trackCount := copyREOFixtures(t, staging)

	releases := newE2EMusicReleaseRepo()
	scanGroups := newE2EMusicScanGroupRepo()
	items := newE2EItemRepo()
	mediaFiles := newMediaFileRepo()

	svc := newRealMusicPipeline(releases, scanGroups, items, mediaFiles, &reoSource{withBarcode: false})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	go func() {
		if err := svc.StartWatching(ctx, []scan.Module{
			{ContentType: domain.ContentTypeMusic, Roots: []string{root}},
		}); err != nil && ctx.Err() == nil {
			t.Errorf("StartWatching: %v", err)
		}
	}()

	time.Sleep(200 * time.Millisecond) // let fsnotify register the root watch

	// Atomic rename into the watched root — mirrors "drop folder" on same-volume
	// Finder drag or a download client's completed-file move. The watcher only
	// fires for filesystem events after it starts watching; pre-existing files
	// are never seen unless a directory-create event triggers the walk-and-emit path.
	dest := filepath.Join(root, "Hi Infidelity")
	if err := os.Rename(staging, dest); err != nil {
		t.Fatal(err)
	}

	group := waitForScanGroup(t, scanGroups, 45*time.Second)

	if group.TotalTracks != trackCount {
		t.Errorf("TotalTracks = %d, want %d (duplicate watch events must collapse per path)", group.TotalTracks, trackCount)
	}
	if group.Tags.AlbumArtist != "REO Speedwagon" {
		t.Errorf("Tags.AlbumArtist = %q, want %q (artist-tag fallback)", group.Tags.AlbumArtist, "REO Speedwagon")
	}
	if group.Status != domain.UnmatchedPending {
		t.Errorf("Status = %q, want pending (confidence must stay below auto-import threshold for untagged rips)", group.Status)
	}
	if len(group.Candidates) == 0 {
		t.Fatal("Candidates is empty — fuzzy artist/album resolution failed")
	}
	if group.Candidates[0].ReleaseGroupTitle != "Hi Infidelity" {
		t.Errorf("top candidate = %q, want %q", group.Candidates[0].ReleaseGroupTitle, "Hi Infidelity")
	}
	if group.Candidates[0].OverallConfidence >= 0.85 {
		t.Errorf("OverallConfidence = %v, want < 0.85 (no barcode/ISRC/track-count tags on these fixtures)", group.Candidates[0].OverallConfidence)
	}

	// A second discovery pass (simulating a re-scan / duplicate directory event)
	// must update the same entry, never create a second one.
	if len(scanGroups.all()) != 1 {
		t.Fatalf("scan groups = %d, want 1 after first pass", len(scanGroups.all()))
	}
}

// TestE2E_RealMusicWatch_WithBarcode_AutoImportsAndWritesBackTags exercises the
// full real pipeline through auto-import: real watcher, real grouper, real
// album identifier reaching the barcode short-circuit, the real ReleaseImporter
// creating real Items/MediaFiles with real SHA1s, and the real MusicTagWriter
// mutating the actual FLAC files on disk — verified by re-reading them.
func TestE2E_RealMusicWatch_WithBarcode_AutoImportsAndWritesBackTags(t *testing.T) {
	root := t.TempDir()
	staging := t.TempDir()
	trackCount := copyREOFixtures(t, staging)

	stagedFiles, _ := filepath.Glob(filepath.Join(staging, "*.flac"))
	for _, m := range stagedFiles {
		addVorbisComment(t, m, "BARCODE", reoBarcode)
	}

	releases := newE2EMusicReleaseRepo()
	releases.seed(reoReleaseMBID, &domain.MusicRelease{
		ID:             "internal-release-id",
		GroupID:        "internal-group-id",
		LibraryEntryID: "internal-entry-id",
		Title:          "Hi Infidelity",
		Status:         domain.ReleaseStatusStub,
	})
	scanGroups := newE2EMusicScanGroupRepo()
	items := newE2EItemRepo()
	mediaFiles := newMediaFileRepo()

	svc := newRealMusicPipeline(releases, scanGroups, items, mediaFiles, &reoSource{withBarcode: true})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	go func() {
		if err := svc.StartWatching(ctx, []scan.Module{
			{ContentType: domain.ContentTypeMusic, Roots: []string{root}},
		}); err != nil && ctx.Err() == nil {
			t.Errorf("StartWatching: %v", err)
		}
	}()

	time.Sleep(200 * time.Millisecond) // let fsnotify register the root watch

	dest := filepath.Join(root, "Hi Infidelity")
	if err := os.Rename(staging, dest); err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(dest, "*.flac"))

	deadline := time.Now().Add(45 * time.Second)
	for releases.savedCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}

	rel, err := releases.GetByMBID(context.Background(), reoReleaseMBID)
	if err != nil {
		t.Fatalf("GetByMBID: %v", err)
	}
	if rel.Status != domain.ReleaseStatusImported {
		t.Fatalf("release.Status = %q, want imported (auto-import via barcode short-circuit never fired)", rel.Status)
	}

	allItems, total, err := items.List(context.Background(), ports.ItemFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != trackCount {
		t.Errorf("items created = %d, want %d (duplicate watch events must not double-import tracks)", total, trackCount)
	}

	for _, it := range allItems {
		if it.MediaFile == nil {
			t.Errorf("item %q has no MediaFile", it.Title)
			continue
		}
		if it.MediaFile.SHA1 == "" {
			t.Errorf("item %q MediaFile.SHA1 is empty, want a real hash", it.Title)
		}
		if it.Metadata["release_id"] != "internal-release-id" {
			t.Errorf("item %q Metadata[release_id] = %v, want internal-release-id", it.Title, it.Metadata["release_id"])
		}
	}

	// The real MusicTagWriter must have written MUSICBRAINZ_ALBUMID back into
	// every file on disk — verified by re-reading the file, not by inspecting
	// in-memory state.
	for _, m := range matches {
		got := readVorbisTag(t, m, "MUSICBRAINZ_ALBUMID")
		if len(got) != 1 || got[0] != reoReleaseMBID {
			t.Errorf("%s: MUSICBRAINZ_ALBUMID = %v, want [%s]", filepath.Base(m), got, reoReleaseMBID)
		}
	}
}
