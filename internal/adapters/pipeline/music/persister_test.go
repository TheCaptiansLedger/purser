package music_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	dsbadger "purser/internal/adapters/datastore/badger"
	imagestorelocal "purser/internal/adapters/imagestore/local"
	"purser/internal/adapters/pipeline/music"
	"purser/internal/adapters/store/externalid"
	"purser/internal/adapters/store/group"
	storeimage "purser/internal/adapters/store/image"
	"purser/internal/adapters/store/item"
	"purser/internal/adapters/store/libraryentry"
	"purser/internal/adapters/store/mediafile"
	storemusic "purser/internal/adapters/store/music"
	"purser/internal/domain"
	musicdomain "purser/internal/domain/music"
	"purser/internal/ports"
	"sync"
	"testing"
)

// fakeJPEGBytes sniffs as image/jpeg via http.DetectContentType's exact-sig
// match on the JPEG SOI marker — enough for imagestore/local's extension
// detection, no real image data needed.
var fakeJPEGBytes = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}

// persisterDeps bundles a Persister backed by real Badger-backed
// repositories (needed to genuinely exercise get-or-create races,
// GetByHash, and Item.Metadata's real JSON round-trip — an in-memory fake
// would silently hide the int/float64 disc_number decoding issue a real
// Datastore round-trip produces) plus a fakeMusicBrainz double.
// fakeOrganizer is a ports.Organizer test double recording every
// mediaFileID it was called with, optionally returning err — used to
// assert the Persister calls it exactly once per created MediaFile, and
// that an organizer error never fails Persist.
type fakeOrganizer struct {
	mu          sync.Mutex
	calls       []string
	organizeErr error
}

func newFakeOrganizer() *fakeOrganizer { return &fakeOrganizer{} }

func (f *fakeOrganizer) Organize(_ context.Context, mediaFileID string) (*domain.MediaFile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, mediaFileID)
	if f.organizeErr != nil {
		return nil, f.organizeErr
	}
	return &domain.MediaFile{ID: mediaFileID}, nil
}

func (f *fakeOrganizer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

type persisterDeps struct {
	mb             *fakeMusicBrainz
	externalIDs    ports.ExternalIDRepository
	libraryEntries ports.LibraryEntryRepository
	groups         ports.GroupRepository
	releases       ports.MusicReleaseRepository
	items          ports.ItemRepository
	mediaFiles     ports.MediaFileRepository
	images         ports.ImageRepository
	imageStore     ports.ImageStore
	organizer      *fakeOrganizer
	persister      *music.Persister
}

func newPersisterDeps(t *testing.T) *persisterDeps {
	t.Helper()
	db, err := dsbadger.Open(dsbadger.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("badger.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	})
	ds, err := dsbadger.New("test", db)
	if err != nil {
		t.Fatalf("badger.New returned error: %v", err)
	}

	externalIDs, err := externalid.New("test", ds)
	if err != nil {
		t.Fatalf("externalid.New returned error: %v", err)
	}
	libraryEntries, err := libraryentry.New("test", ds)
	if err != nil {
		t.Fatalf("libraryentry.New returned error: %v", err)
	}
	groups, err := group.New("test", ds)
	if err != nil {
		t.Fatalf("group.New returned error: %v", err)
	}
	releases, err := storemusic.New("test", ds)
	if err != nil {
		t.Fatalf("storemusic.New returned error: %v", err)
	}
	items, err := item.New("test", ds)
	if err != nil {
		t.Fatalf("item.New returned error: %v", err)
	}
	mediaFiles, err := mediafile.New("test", ds)
	if err != nil {
		t.Fatalf("mediafile.New returned error: %v", err)
	}
	images, err := storeimage.New("test", ds)
	if err != nil {
		t.Fatalf("storeimage.New returned error: %v", err)
	}
	imageStore, err := imagestorelocal.New("test", t.TempDir())
	if err != nil {
		t.Fatalf("imagestorelocal.New returned error: %v", err)
	}

	mb := newFakeMusicBrainz()
	organizer := newFakeOrganizer()
	persister := music.NewPersister(mb, externalIDs, libraryEntries, groups, releases, items, mediaFiles, images, imageStore, organizer)

	return &persisterDeps{
		mb: mb, externalIDs: externalIDs, libraryEntries: libraryEntries, groups: groups,
		releases: releases, items: items, mediaFiles: mediaFiles, images: images, imageStore: imageStore,
		organizer: organizer, persister: persister,
	}
}

// hiInfidelityFixture wires up REO Speedwagon's real "Hi Infidelity"
// MusicBrainz release into a persisterDeps' fakeMusicBrainz — loaded
// directly from a real recorded response
// (testdata/release_lookup_hi_infidelity.json,
// testdata/artist_lookup_reo_speedwagon.json), not hand-typed field
// values. This is exactly what purser#522's manual verification used to
// catch two real, previously-invisible bugs that a guessed fixture had
// accidentally matched around: a raw TRACKNUMBER tag's zero-padding
// ("01") never string-equals MusicBrainz's own unpadded track.Number
// ("1"), and MusicBrainz's medium.position is always >= 1 (never 0,
// even for a single-disc release's only disc) while Purser's own
// DiscNumber defaults to 0 whenever nothing gave it a real value — see
// findUnmatchedFile's own doc comment for the fix. Trimmed to the
// release's first 2 tracks (real values, just fewer of them) to match
// track1File/track2File below. Returns the real release MBID.
func hiInfidelityFixture(t *testing.T, d *persisterDeps) string {
	t.Helper()

	var release ports.Release
	loadTestdataJSON(t, "release_lookup_hi_infidelity.json", &release)
	var artist ports.Artist
	loadTestdataJSON(t, "artist_lookup_reo_speedwagon.json", &artist)

	if len(release.Media) != 1 || len(release.Media[0].Tracks) < 2 || release.ReleaseGroup == nil {
		t.Fatalf("testdata/release_lookup_hi_infidelity.json shape changed unexpectedly: %+v", release)
	}
	release.Media[0].Tracks = release.Media[0].Tracks[:2]

	d.mb.artists[artist.ID] = artist
	d.mb.releaseGroups[release.ReleaseGroup.ID] = *release.ReleaseGroup
	d.mb.releases[release.ID] = release
	return release.ID
}

// loadTestdataJSON decodes testdata/name (a real recorded MusicBrainz
// response — see hiInfidelityFixture) into out, failing the test on any
// error.
func loadTestdataJSON(t *testing.T, name string, out any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading testdata/%s: %v", name, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decoding testdata/%s: %v", name, err)
	}
}

// track1File and track2File model the two files of the "Hi Infidelity"
// fixture living in dir — a real directory (not just a string) since
// Persist's cover-art step (M10b) does a genuine os.ReadDir against
// GroupKey. DiscNumber/TrackNumber are the exact values Purser's own
// scan pipeline produced for the real Hi Infidelity FLAC fixture
// (test-data/music/1980 Hi Infidelity), captured live during purser#522's
// manual verification via ListUnmatchedFiles — not guessed:
//
//   - TrackNumber "01"/"02" — zero-padded, the raw TRACKNUMBER tag value
//     ffprobe read straight off the file. hiInfidelityFixture's real
//     MusicBrainz data reports the same tracks as unpadded "1"/"2" — a
//     real formatting gap findUnmatchedFile's old naive string equality
//     missed entirely.
//   - DiscNumber 0 — no DISCNUMBER tag on the file at all (there's only
//     one disc; nothing to tag), Purser's own "no disc info given"
//     sentinel. MusicBrainz's medium.position is always >= 1, never 0,
//     even for a single-disc release's only disc — another real gap
//     findUnmatchedFile missed, and the more consequential one: it broke
//     positional matching for every ordinary single-disc album with no
//     explicit disc tag, not just an edge case.
//
// Both gaps are exactly what a hand-typed fixture (DiscNumber: 1,
// TrackNumber matching MusicBrainz's own unpadded value) had accidentally
// matched around before this fixture was corrected to reflect reality —
// see findUnmatchedFile's own doc comment for the fix.
func track1File(dir string) *domain.UnmatchedFile {
	return &domain.UnmatchedFile{
		ID: domain.NewID(), Path: filepath.Join(dir, "01 Don't Let Him Go.flac"), ContentType: domain.ContentTypeMusic,
		GroupKey: dir, DiscNumber: 0, TrackNumber: "01",
		OSHash: "oshash-track1", SHA1: "sha1-track1", Status: domain.UnmatchedFileStatusPending,
	}
}

func track2File(dir string) *domain.UnmatchedFile {
	return &domain.UnmatchedFile{
		ID: domain.NewID(), Path: filepath.Join(dir, "02 Keep on Loving You.flac"), ContentType: domain.ContentTypeMusic,
		GroupKey: dir, DiscNumber: 0, TrackNumber: "02",
		OSHash: "oshash-track2", SHA1: "sha1-track2", Status: domain.UnmatchedFileStatusPending,
	}
}

func TestPersister_Persist_HappyPath(t *testing.T) {
	d := newPersisterDeps(t)
	releaseMBID := hiInfidelityFixture(t, d)
	dir := t.TempDir()
	files := []*domain.UnmatchedFile{track1File(dir), track2File(dir)}
	candidate := domain.MatchCandidate{ExternalRef: releaseMBID, Tier: domain.MatchTierDirectID, Score: 0.98}

	if err := d.persister.Persist(context.Background(), nil, candidate, files); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}

	release, err := d.releases.GetByMBID(context.Background(), releaseMBID)
	if err != nil {
		t.Fatalf("GetByMBID returned error: %v", err)
	}
	if release.Status != musicdomain.ReleaseStatusImported {
		t.Fatalf("release.Status = %q, want %q", release.Status, musicdomain.ReleaseStatusImported)
	}
	if release.TrackCount != 2 || release.MediumCount != 1 {
		t.Fatalf("release TrackCount/MediumCount = %d/%d, want 2/1", release.TrackCount, release.MediumCount)
	}

	entry, err := d.libraryEntries.Get(context.Background(), release.LibraryEntryID)
	if err != nil {
		t.Fatalf("Get LibraryEntry returned error: %v", err)
	}
	if entry.Name != "REO Speedwagon" || entry.Kind != domain.KindArtist {
		t.Fatalf("LibraryEntry = %+v, want Name=REO Speedwagon Kind=artist", entry)
	}

	grp, err := d.groups.Get(context.Background(), release.GroupID)
	if err != nil {
		t.Fatalf("Get Group returned error: %v", err)
	}
	if grp.Title != "Hi Infidelity" || grp.LibraryEntryID != release.LibraryEntryID {
		t.Fatalf("Group = %+v, want Title=Hi Infidelity LibraryEntryID=%q", grp, release.LibraryEntryID)
	}

	tracks, _, err := d.releases.ListTracksByRelease(context.Background(), release.ID, 10, "")
	if err != nil {
		t.Fatalf("ListTracksByRelease returned error: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("ListTracksByRelease returned %d tracks, want 2", len(tracks))
	}
	for _, tr := range tracks {
		if tr.Status != domain.ItemStatusImported {
			t.Errorf("track %q Status = %q, want %q", tr.Title, tr.Status, domain.ItemStatusImported)
		}
		if tr.LibraryEntryID != release.LibraryEntryID {
			t.Errorf("track %q LibraryEntryID = %q, want %q (the resolved Artist, per the issue's explicit requirement)", tr.Title, tr.LibraryEntryID, release.LibraryEntryID)
		}
		if tr.GroupID != release.GroupID {
			t.Errorf("track %q GroupID = %q, want %q", tr.Title, tr.GroupID, release.GroupID)
		}
	}
}

func TestPersister_Persist_CallsOrganizerOncePerCreatedMediaFile(t *testing.T) {
	d := newPersisterDeps(t)
	releaseMBID := hiInfidelityFixture(t, d)
	dir := t.TempDir()
	files := []*domain.UnmatchedFile{track1File(dir), track2File(dir)}
	candidate := domain.MatchCandidate{ExternalRef: releaseMBID}

	if err := d.persister.Persist(context.Background(), nil, candidate, files); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}

	if got := d.organizer.callCount(); got != 2 {
		t.Fatalf("organizer was called %d times, want 2 (once per created MediaFile)", got)
	}
}

func TestPersister_Persist_OrganizerErrorIsLoggedAndSwallowed(t *testing.T) {
	d := newPersisterDeps(t)
	d.organizer.organizeErr = errors.New("simulated organize failure")
	releaseMBID := hiInfidelityFixture(t, d)
	dir := t.TempDir()
	files := []*domain.UnmatchedFile{track1File(dir), track2File(dir)}
	candidate := domain.MatchCandidate{ExternalRef: releaseMBID}

	if err := d.persister.Persist(context.Background(), nil, candidate, files); err != nil {
		t.Fatalf("Persist returned error %v, want nil — an organizer failure must never fail Persist", err)
	}

	release, err := d.releases.GetByMBID(context.Background(), releaseMBID)
	if err != nil {
		t.Fatalf("GetByMBID returned error: %v", err)
	}
	if release.Status != musicdomain.ReleaseStatusImported {
		t.Fatalf("release.Status = %q, want %q despite the organizer failure", release.Status, musicdomain.ReleaseStatusImported)
	}
	if got := d.organizer.callCount(); got != 2 {
		t.Fatalf("organizer was called %d times, want 2", got)
	}
}

func TestPersister_Persist_PartialWhenATrackHasNoFile(t *testing.T) {
	d := newPersisterDeps(t)
	releaseMBID := hiInfidelityFixture(t, d)
	dir := t.TempDir()
	files := []*domain.UnmatchedFile{track1File(dir)} // track 2 has no file
	candidate := domain.MatchCandidate{ExternalRef: releaseMBID}

	if err := d.persister.Persist(context.Background(), nil, candidate, files); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}

	release, err := d.releases.GetByMBID(context.Background(), releaseMBID)
	if err != nil {
		t.Fatalf("GetByMBID returned error: %v", err)
	}
	if release.Status != musicdomain.ReleaseStatusPartial {
		t.Fatalf("release.Status = %q, want %q", release.Status, musicdomain.ReleaseStatusPartial)
	}

	tracks, _, err := d.releases.ListTracksByRelease(context.Background(), release.ID, 10, "")
	if err != nil {
		t.Fatalf("ListTracksByRelease returned error: %v", err)
	}
	var missing, imported int
	for _, tr := range tracks {
		switch tr.Status {
		case domain.ItemStatusMissing:
			missing++
		case domain.ItemStatusImported:
			imported++
		}
	}
	if missing != 1 || imported != 1 {
		t.Fatalf("tracks missing/imported = %d/%d, want 1/1", missing, imported)
	}
}

// failNthMediaFileCreate wraps a real ports.MediaFileRepository and fails
// the Nth call to Create — used to simulate a partial failure partway
// through persistTracks' loop.
type failNthMediaFileCreate struct {
	ports.MediaFileRepository
	failOnCall int
	calls      int
}

func (f *failNthMediaFileCreate) Create(ctx context.Context, m *domain.MediaFile) error {
	f.calls++
	if f.calls == f.failOnCall {
		return errors.New("simulated failure")
	}
	return f.MediaFileRepository.Create(ctx, m)
}

// TestPersister_Persist_RetryAfterPartialFailureDoesNotDuplicate is the
// load-bearing test for the design-review gap the issue calls out by
// name: a retry after a partial failure partway through the Items/
// MediaFiles loop must not duplicate the tracks that already succeeded.
func TestPersister_Persist_RetryAfterPartialFailureDoesNotDuplicate(t *testing.T) {
	d := newPersisterDeps(t)
	releaseMBID := hiInfidelityFixture(t, d)
	dir := t.TempDir()
	files := []*domain.UnmatchedFile{track1File(dir), track2File(dir)}
	candidate := domain.MatchCandidate{ExternalRef: releaseMBID}

	failing := &failNthMediaFileCreate{MediaFileRepository: d.mediaFiles, failOnCall: 2}
	failingPersister := music.NewPersister(d.mb, d.externalIDs, d.libraryEntries, d.groups, d.releases, d.items, failing, d.images, d.imageStore, newFakeOrganizer())

	err := failingPersister.Persist(context.Background(), nil, candidate, files)
	if err == nil {
		t.Fatal("Persist with a forced MediaFile.Create failure returned nil, want an error")
	}
	if failing.calls != 2 {
		t.Fatalf("forced failure hit %d MediaFile.Create calls, want exactly 2 (track 1 succeeds, track 2 fails)", failing.calls)
	}

	// Retry with a Persister backed by the real (non-failing) MediaFile
	// repository — same underlying storage, so this exercises genuine
	// resume-from-partial-failure, not a fresh attempt.
	if err := d.persister.Persist(context.Background(), nil, candidate, files); err != nil {
		t.Fatalf("retry Persist returned error: %v", err)
	}

	release, err := d.releases.GetByMBID(context.Background(), releaseMBID)
	if err != nil {
		t.Fatalf("GetByMBID returned error: %v", err)
	}
	if release.Status != musicdomain.ReleaseStatusImported {
		t.Fatalf("release.Status after retry = %q, want %q", release.Status, musicdomain.ReleaseStatusImported)
	}

	tracks, _, err := d.releases.ListTracksByRelease(context.Background(), release.ID, 10, "")
	if err != nil {
		t.Fatalf("ListTracksByRelease returned error: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("ListTracksByRelease after retry returned %d tracks, want exactly 2 (no duplicates)", len(tracks))
	}
	for _, tr := range tracks {
		if tr.Status != domain.ItemStatusImported {
			t.Errorf("track %q Status after retry = %q, want %q", tr.Title, tr.Status, domain.ItemStatusImported)
		}
		mfs, _, err := d.mediaFiles.List(context.Background(), tr.ID, 10, "")
		if err != nil {
			t.Fatalf("MediaFile List for item %s returned error: %v", tr.ID, err)
		}
		if len(mfs) != 1 {
			t.Errorf("track %q has %d MediaFiles, want exactly 1 (no duplicates)", tr.Title, len(mfs))
		}
	}
}

func TestPersister_Persist_ConcurrentSameArtistLandsOnOneLibraryEntry(t *testing.T) {
	d := newPersisterDeps(t)

	const artistMBID = "artist-shared"
	d.mb.artists[artistMBID] = ports.Artist{ID: artistMBID, Name: "Shared Artist", Type: "Group"}

	releaseMBIDs := make([]string, 2)
	for i := range 2 {
		rgMBID := fmt.Sprintf("rg-%d", i)
		releaseMBID := fmt.Sprintf("release-%d", i)
		releaseMBIDs[i] = releaseMBID
		d.mb.releaseGroups[rgMBID] = ports.ReleaseGroup{ID: rgMBID, Title: fmt.Sprintf("Album %d", i), PrimaryType: "Album"}
		d.mb.releases[releaseMBID] = ports.Release{
			ID: releaseMBID, Title: fmt.Sprintf("Album %d", i),
			ReleaseGroup: &ports.ReleaseGroup{ID: rgMBID, Title: fmt.Sprintf("Album %d", i), PrimaryType: "Album"},
			ArtistCredit: []ports.ArtistCredit{{Name: "Shared Artist", Artist: ports.RelationArtist{ID: artistMBID, Name: "Shared Artist"}}},
		}
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			candidate := domain.MatchCandidate{ExternalRef: releaseMBIDs[i]}
			errs[i] = d.persister.Persist(context.Background(), nil, candidate, nil)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Persist[%d] returned error: %v", i, err)
		}
	}

	r0, err := d.releases.GetByMBID(context.Background(), releaseMBIDs[0])
	if err != nil {
		t.Fatalf("GetByMBID(0) returned error: %v", err)
	}
	r1, err := d.releases.GetByMBID(context.Background(), releaseMBIDs[1])
	if err != nil {
		t.Fatalf("GetByMBID(1) returned error: %v", err)
	}
	if r0.LibraryEntryID != r1.LibraryEntryID {
		t.Fatalf("concurrent Persist for the same artist landed on two LibraryEntrys: %q and %q, want one", r0.LibraryEntryID, r1.LibraryEntryID)
	}

	entries, _, err := d.libraryEntries.List(context.Background(), domain.KindArtist, "", 10, "")
	if err != nil {
		t.Fatalf("List LibraryEntries returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("List LibraryEntries returned %d rows, want exactly 1 (the race must not create two)", len(entries))
	}
}

// writeFixtureImage writes fakeJPEGBytes to dir/name, failing the test on
// error.
func writeFixtureImage(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), fakeJPEGBytes, 0o600); err != nil {
		t.Fatalf("writing fixture image %s: %v", name, err)
	}
}

// TestPersister_Persist_CoverArtRankingAndAttachment is the M10b happy
// path: cover.jpg/folder.jpg/front.jpg (plus a non-convention "back.jpg"
// and a non-image sidecar) sitting in the group's folder all attach as
// Image rows, ranked cover > folder > front/album > other, and the actual
// bytes round-trip back out of ImageStore — per issue #520's verification
// checklist.
func TestPersister_Persist_CoverArtRankingAndAttachment(t *testing.T) {
	d := newPersisterDeps(t)
	releaseMBID := hiInfidelityFixture(t, d)
	dir := t.TempDir()
	files := []*domain.UnmatchedFile{track1File(dir), track2File(dir)}
	candidate := domain.MatchCandidate{ExternalRef: releaseMBID}

	// Deliberately written out of rank order, to prove ranking (not
	// directory-listing order) decides Priority.
	writeFixtureImage(t, dir, "back.jpg")
	writeFixtureImage(t, dir, "front.jpg")
	writeFixtureImage(t, dir, "folder.jpg")
	writeFixtureImage(t, dir, "cover.jpg")
	if err := os.WriteFile(filepath.Join(dir, "album.nfo"), []byte("not an image"), 0o600); err != nil {
		t.Fatalf("writing non-image sidecar: %v", err)
	}

	if err := d.persister.Persist(context.Background(), nil, candidate, files); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}

	release, err := d.releases.GetByMBID(context.Background(), releaseMBID)
	if err != nil {
		t.Fatalf("GetByMBID returned error: %v", err)
	}

	images, _, err := d.images.List(context.Background(), "music_release", release.ID, 10, "")
	if err != nil {
		t.Fatalf("images.List returned error: %v", err)
	}
	if len(images) != 4 {
		t.Fatalf("images.List returned %d images, want 4 (album.nfo must not attach)", len(images))
	}

	byPriority := make(map[int]*domain.Image, len(images))
	for _, img := range images {
		if existing, dup := byPriority[img.Priority]; dup {
			t.Fatalf("two images share Priority %d: %+v and %+v", img.Priority, existing, img)
		}
		byPriority[img.Priority] = img
	}
	for rank := range 4 {
		img, ok := byPriority[rank]
		if !ok {
			t.Fatalf("no image found with Priority %d", rank)
		}
		if img.OwnerType != "music_release" || img.OwnerID != release.ID {
			t.Errorf("image at Priority %d has OwnerType/OwnerID = %q/%q, want music_release/%q", rank, img.OwnerType, img.OwnerID, release.ID)
		}
		if img.ImageType != domain.ImageTypePoster {
			t.Errorf("image at Priority %d has ImageType = %q, want %q", rank, img.ImageType, domain.ImageTypePoster)
		}

		rc, err := d.imageStore.Get(context.Background(), img.URL)
		if err != nil {
			t.Fatalf("imageStore.Get(%q) returned error: %v", img.URL, err)
		}
		got := make([]byte, len(fakeJPEGBytes))
		if _, err := rc.Read(got); err != nil {
			t.Fatalf("reading image bytes returned error: %v", err)
		}
		rc.Close()
		for i, b := range fakeJPEGBytes {
			if got[i] != b {
				t.Fatalf("image bytes at Priority %d = %v, want %v", rank, got, fakeJPEGBytes)
			}
		}
	}
}

// TestPersister_Persist_CoverArtInSubfolderAttaches is a regression test
// for purser#522's manual verification finding: a real box set's cover
// art commonly lives in its own subfolder ("Covers"/"Scans"/"Artwork") or
// inside a per-disc subfolder, not loose next to the audio — the exact
// shape of the real Hi Infidelity fixture (test-data/music/1980 Hi
// Infidelity/Covers/*.png). persistCoverArt used to be a flat,
// single-level os.ReadDir of the group's own folder only, so a subfolder
// like this attached zero images, every time. It's now a recursive walk
// (see persistCoverArt's own doc comment) — this proves an image nested
// two levels down (dir/Artwork/Scans/cover.jpg) is found and attached.
func TestPersister_Persist_CoverArtInSubfolderAttaches(t *testing.T) {
	d := newPersisterDeps(t)
	releaseMBID := hiInfidelityFixture(t, d)
	dir := t.TempDir()
	files := []*domain.UnmatchedFile{track1File(dir), track2File(dir)}
	candidate := domain.MatchCandidate{ExternalRef: releaseMBID}

	nested := filepath.Join(dir, "Artwork", "Scans")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatalf("creating nested fixture dir: %v", err)
	}
	writeFixtureImage(t, nested, "cover.jpg")

	if err := d.persister.Persist(context.Background(), nil, candidate, files); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}

	release, err := d.releases.GetByMBID(context.Background(), releaseMBID)
	if err != nil {
		t.Fatalf("GetByMBID returned error: %v", err)
	}
	images, _, err := d.images.List(context.Background(), "music_release", release.ID, 10, "")
	if err != nil {
		t.Fatalf("images.List returned error: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("images.List returned %d images, want 1 (the nested Artwork/Scans/cover.jpg)", len(images))
	}
}

// TestPersister_Persist_CoverArtRetryDoesNotDuplicate confirms the
// duplicate-attachment guard: re-running Persist against an
// already-imported release (simulating a retry) must not create additional
// Image rows.
func TestPersister_Persist_CoverArtRetryDoesNotDuplicate(t *testing.T) {
	d := newPersisterDeps(t)
	releaseMBID := hiInfidelityFixture(t, d)
	dir := t.TempDir()
	files := []*domain.UnmatchedFile{track1File(dir), track2File(dir)}
	candidate := domain.MatchCandidate{ExternalRef: releaseMBID}
	writeFixtureImage(t, dir, "cover.jpg")

	if err := d.persister.Persist(context.Background(), nil, candidate, files); err != nil {
		t.Fatalf("first Persist returned error: %v", err)
	}
	if err := d.persister.Persist(context.Background(), nil, candidate, files); err != nil {
		t.Fatalf("retry Persist returned error: %v", err)
	}

	release, err := d.releases.GetByMBID(context.Background(), releaseMBID)
	if err != nil {
		t.Fatalf("GetByMBID returned error: %v", err)
	}
	images, _, err := d.images.List(context.Background(), "music_release", release.ID, 10, "")
	if err != nil {
		t.Fatalf("images.List returned error: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("images.List after retry returned %d images, want exactly 1 (no duplicates)", len(images))
	}
}

// TestPersister_Persist_NoImagesInFolderAttachesNothing confirms a group
// folder with no recognized image files leaves the release with zero
// Image rows — the ordinary case for most releases, and proof
// persistCoverArt doesn't error when there's simply nothing to attach.
func TestPersister_Persist_NoImagesInFolderAttachesNothing(t *testing.T) {
	d := newPersisterDeps(t)
	releaseMBID := hiInfidelityFixture(t, d)
	dir := t.TempDir()
	files := []*domain.UnmatchedFile{track1File(dir), track2File(dir)}
	candidate := domain.MatchCandidate{ExternalRef: releaseMBID}

	if err := d.persister.Persist(context.Background(), nil, candidate, files); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}

	release, err := d.releases.GetByMBID(context.Background(), releaseMBID)
	if err != nil {
		t.Fatalf("GetByMBID returned error: %v", err)
	}
	images, _, err := d.images.List(context.Background(), "music_release", release.ID, 10, "")
	if err != nil {
		t.Fatalf("images.List returned error: %v", err)
	}
	if len(images) != 0 {
		t.Fatalf("images.List returned %d images, want 0", len(images))
	}
}

// TestPersister_Persist_VariousArtistsFirstImportNeedsNoSeedStep confirms
// ADR-0021's decision: importing a VA-credited release with no prior seed
// step just resolves the sentinel artist through the ordinary get-or-create
// path, same as any other artist.
func TestPersister_Persist_VariousArtistsFirstImportNeedsNoSeedStep(t *testing.T) {
	d := newPersisterDeps(t)
	const vaMBID = "89ad4ac3-39f7-470e-963a-56509c546377" // MusicBrainz's real "Various Artists" MBID
	const rgMBID = "rg-va-compilation"
	const releaseMBID = "release-va-compilation"

	d.mb.artists[vaMBID] = ports.Artist{ID: vaMBID, Name: "Various Artists", Type: "Other"}
	d.mb.releaseGroups[rgMBID] = ports.ReleaseGroup{ID: rgMBID, Title: "Now That's What I Call Music!", PrimaryType: "Album", SecondaryTypes: []string{"Compilation"}}
	d.mb.releases[releaseMBID] = ports.Release{
		ID: releaseMBID, Title: "Now That's What I Call Music!",
		ReleaseGroup: &ports.ReleaseGroup{ID: rgMBID, Title: "Now That's What I Call Music!", PrimaryType: "Album", SecondaryTypes: []string{"Compilation"}},
		ArtistCredit: []ports.ArtistCredit{{Name: "Various Artists", Artist: ports.RelationArtist{ID: vaMBID, Name: "Various Artists"}}},
	}

	candidate := domain.MatchCandidate{ExternalRef: releaseMBID}
	if err := d.persister.Persist(context.Background(), nil, candidate, nil); err != nil {
		t.Fatalf("Persist returned error: %v", err)
	}

	release, err := d.releases.GetByMBID(context.Background(), releaseMBID)
	if err != nil {
		t.Fatalf("GetByMBID returned error: %v", err)
	}
	entry, err := d.libraryEntries.Get(context.Background(), release.LibraryEntryID)
	if err != nil {
		t.Fatalf("Get LibraryEntry returned error: %v", err)
	}
	if entry.Name != "Various Artists" {
		t.Fatalf("LibraryEntry.Name = %q, want %q", entry.Name, "Various Artists")
	}

	grp, err := d.groups.Get(context.Background(), release.GroupID)
	if err != nil {
		t.Fatalf("Get Group returned error: %v", err)
	}
	if grp.Metadata["album_type"] != "compilation" {
		t.Fatalf("Group.Metadata[album_type] = %v, want %q", grp.Metadata["album_type"], "compilation")
	}
}
