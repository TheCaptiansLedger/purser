package music_test

import (
	"context"
	"errors"
	"purser/internal/app/errs"
	"purser/internal/app/music"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// ── mocks ─────────────────────────────────────────────────────────────────────

type mockReleaseRepo struct {
	byMBID map[string]*domain.MusicRelease
	saved  []*domain.MusicRelease
}

func newMockReleaseRepo(mbid string, rel *domain.MusicRelease) *mockReleaseRepo {
	return &mockReleaseRepo{byMBID: map[string]*domain.MusicRelease{mbid: rel}}
}

func (m *mockReleaseRepo) Get(_ context.Context, _ string) (*domain.MusicRelease, error) {
	return nil, errs.ErrNotFound
}

func (m *mockReleaseRepo) GetByMBID(_ context.Context, mbid string) (*domain.MusicRelease, error) {
	if r, ok := m.byMBID[mbid]; ok {
		return r, nil
	}
	return nil, errs.ErrNotFound
}

func (m *mockReleaseRepo) GetByBarcode(_ context.Context, _ string) (*domain.MusicRelease, error) {
	return nil, errs.ErrNotFound
}

func (m *mockReleaseRepo) ListByGroup(_ context.Context, _ string) ([]*domain.MusicRelease, error) {
	return nil, nil
}

func (m *mockReleaseRepo) ListByEntry(_ context.Context, _ string) ([]*domain.MusicRelease, error) {
	return nil, nil
}

func (m *mockReleaseRepo) ListTracksByRelease(_ context.Context, _ string) ([]*domain.Item, error) {
	return nil, nil
}

func (m *mockReleaseRepo) Save(_ context.Context, r *domain.MusicRelease) error {
	m.saved = append(m.saved, r)
	return nil
}

func (m *mockReleaseRepo) Delete(_ context.Context, _ string) error { return nil }

type mockItemRepo struct {
	saved []*domain.Item
}

func (m *mockItemRepo) Get(_ context.Context, _ string) (*domain.Item, error) {
	return nil, errs.ErrNotFound
}

func (m *mockItemRepo) List(_ context.Context, _ ports.ItemFilter) ([]*domain.Item, int, error) {
	return nil, 0, nil
}

func (m *mockItemRepo) Save(_ context.Context, item *domain.Item) error {
	m.saved = append(m.saved, item)
	return nil
}

func (m *mockItemRepo) Delete(_ context.Context, _ string) error        { return nil }
func (m *mockItemRepo) DeleteByGroup(_ context.Context, _ string) error { return nil }
func (m *mockItemRepo) DeleteByLibraryEntry(_ context.Context, _ string) error {
	return nil
}

func (m *mockItemRepo) DeletionImpact(_ context.Context, _ string) (*domain.DeletionImpact, error) {
	return &domain.DeletionImpact{}, nil
}

type mockMediaFileRepo struct {
	saved []*domain.MediaFile
}

func (m *mockMediaFileRepo) GetByItemID(_ context.Context, _ string) (*domain.MediaFile, error) {
	return nil, errs.ErrNotFound
}

func (m *mockMediaFileRepo) GetByOSHash(_ context.Context, _ string) (*domain.MediaFile, error) {
	return nil, errs.ErrNotFound
}

func (m *mockMediaFileRepo) GetByPath(_ context.Context, _ string) (*domain.MediaFile, error) {
	return nil, errs.ErrNotFound
}

func (m *mockMediaFileRepo) Save(_ context.Context, f *domain.MediaFile) error {
	m.saved = append(m.saved, f)
	return nil
}

func (m *mockMediaFileRepo) Delete(_ context.Context, _ string) error { return nil }

type mockFileSystem struct {
	sha1    string
	sha1Err error
}

func (m *mockFileSystem) Stat(_ context.Context, _ string) (*ports.FileInfo, error) {
	return nil, errs.ErrNotFound
}
func (m *mockFileSystem) Move(_ context.Context, _, _ string) error { return nil }
func (m *mockFileSystem) OSHash(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (m *mockFileSystem) SHA1(_ context.Context, _ string) (string, error) {
	return m.sha1, m.sha1Err
}

func (m *mockFileSystem) Walk(_ context.Context, _ string, _ func(ports.FileInfo) error) error {
	return nil
}

type tagWriteCall struct {
	path, releaseMBID, recordingMBID string
}

type mockTagWriter struct {
	calls []tagWriteCall
	err   error
}

func (m *mockTagWriter) WriteIDs(_ context.Context, path, releaseMBID, recordingMBID string) error {
	m.calls = append(m.calls, tagWriteCall{path, releaseMBID, recordingMBID})
	return m.err
}

// ── fixtures ──────────────────────────────────────────────────────────────────

const testReleaseMBID = "release-mbid-abc"

func stubRelease() *domain.MusicRelease {
	return &domain.MusicRelease{
		ID:             "release-id-1",
		GroupID:        "group-1",
		LibraryEntryID: "entry-1",
		Title:          "Hi Infidelity",
		Status:         domain.ReleaseStatusStub,
	}
}

func scannedFile(path string, tags map[string]string) domain.ScannedFile {
	return domain.ScannedFile{
		Path:        path,
		Size:        1000,
		ContentType: domain.ContentTypeMusic,
		Fingerprint: &domain.Fingerprint{EmbeddedTags: tags},
	}
}

func newImporter(releases *mockReleaseRepo, items *mockItemRepo, mediaFiles *mockMediaFileRepo, fsys *mockFileSystem, tagWriter *mockTagWriter) ports.AlbumImporter {
	return music.NewReleaseImporter(releases, items, mediaFiles, fsys, tagWriter)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestReleaseImporter_CreatesAllEntities(t *testing.T) {
	releases := newMockReleaseRepo(testReleaseMBID, stubRelease())
	items := &mockItemRepo{}
	mediaFiles := &mockMediaFileRepo{}
	fsys := &mockFileSystem{sha1: "deadbeef"}
	tagWriter := &mockTagWriter{}

	importer := newImporter(releases, items, mediaFiles, fsys, tagWriter)

	group := ports.ScannedFileGroup{
		RootPath: "/music/Hi Infidelity",
		Files: []domain.ScannedFile{
			scannedFile("/music/Hi Infidelity/01.flac", map[string]string{
				"title": "Don't Let Him Go", "track_number": "1", "disc_number": "1",
				"isrc": "USAB12300001", "duration_ms": "226000", "musicbrainz_track_id": "rec-1",
			}),
			scannedFile("/music/Hi Infidelity/02.flac", map[string]string{
				"title": "Keep On Loving You", "track_number": "2", "disc_number": "1",
				"isrc": "USAB12300002", "duration_ms": "202000", "musicbrainz_track_id": "rec-2",
			}),
		},
	}
	candidate := domain.MusicReleaseCandidate{ReleaseMBID: testReleaseMBID, ReleaseTitle: "Hi Infidelity"}

	if err := importer.ImportRelease(t.Context(), candidate, group); err != nil {
		t.Fatalf("ImportRelease: %v", err)
	}

	if len(items.saved) != 2 {
		t.Fatalf("items saved = %d, want 2", len(items.saved))
	}
	if len(mediaFiles.saved) != 2 {
		t.Fatalf("media files saved = %d, want 2", len(mediaFiles.saved))
	}
	for i, mf := range mediaFiles.saved {
		if mf.SHA1 != "deadbeef" {
			t.Errorf("media file %d SHA1 = %q, want %q", i, mf.SHA1, "deadbeef")
		}
		if mf.ItemID != items.saved[i].ID {
			t.Errorf("media file %d ItemID = %q, want %q", i, mf.ItemID, items.saved[i].ID)
		}
	}
	if items.saved[0].Title != "Don't Let Him Go" {
		t.Errorf("item[0].Title = %q, want %q", items.saved[0].Title, "Don't Let Him Go")
	}
	if items.saved[0].Status != domain.StatusImported {
		t.Errorf("item[0].Status = %q, want %q", items.saved[0].Status, domain.StatusImported)
	}
	if items.saved[0].GroupID != "group-1" || items.saved[0].LibraryEntryID != "entry-1" {
		t.Errorf("item[0] GroupID/LibraryEntryID = %q/%q, want group-1/entry-1", items.saved[0].GroupID, items.saved[0].LibraryEntryID)
	}
}

func TestReleaseImporter_UpdatesStubToImported(t *testing.T) {
	rel := stubRelease()
	releases := newMockReleaseRepo(testReleaseMBID, rel)
	items := &mockItemRepo{}
	mediaFiles := &mockMediaFileRepo{}
	importer := newImporter(releases, items, mediaFiles, &mockFileSystem{}, &mockTagWriter{})

	group := ports.ScannedFileGroup{Files: []domain.ScannedFile{
		scannedFile("/music/01.flac", map[string]string{"track_number": "1"}),
	}}
	candidate := domain.MusicReleaseCandidate{ReleaseMBID: testReleaseMBID}

	if err := importer.ImportRelease(t.Context(), candidate, group); err != nil {
		t.Fatalf("ImportRelease: %v", err)
	}

	if rel.Status != domain.ReleaseStatusImported {
		t.Errorf("release.Status = %q, want %q", rel.Status, domain.ReleaseStatusImported)
	}
	if rel.TrackCount != 1 {
		t.Errorf("release.TrackCount = %d, want 1", rel.TrackCount)
	}
	if len(releases.saved) != 1 {
		t.Errorf("releases.Save called %d times, want 1", len(releases.saved))
	}
}

func TestReleaseImporter_TracksHaveCorrectMetadataKeys(t *testing.T) {
	releases := newMockReleaseRepo(testReleaseMBID, stubRelease())
	items := &mockItemRepo{}
	importer := newImporter(releases, items, &mockMediaFileRepo{}, &mockFileSystem{}, &mockTagWriter{})

	group := ports.ScannedFileGroup{Files: []domain.ScannedFile{
		scannedFile("/music/01.flac", map[string]string{
			"track_number": "1", "disc_number": "2", "isrc": "USAB12300099",
		}),
	}}
	candidate := domain.MusicReleaseCandidate{ReleaseMBID: testReleaseMBID}

	if err := importer.ImportRelease(t.Context(), candidate, group); err != nil {
		t.Fatalf("ImportRelease: %v", err)
	}

	meta := items.saved[0].Metadata
	if meta["release_id"] != "release-id-1" {
		t.Errorf("Metadata[release_id] = %v, want %q", meta["release_id"], "release-id-1")
	}
	if meta["disc_number"] != "2" {
		t.Errorf("Metadata[disc_number] = %v, want %q", meta["disc_number"], "2")
	}
	if meta["isrc"] != "USAB12300099" {
		t.Errorf("Metadata[isrc] = %v, want %q", meta["isrc"], "USAB12300099")
	}
}

func TestReleaseImporter_MultiDisc_DiscNumbersCorrect(t *testing.T) {
	releases := newMockReleaseRepo(testReleaseMBID, stubRelease())
	items := &mockItemRepo{}
	importer := newImporter(releases, items, &mockMediaFileRepo{}, &mockFileSystem{}, &mockTagWriter{})

	group := ports.ScannedFileGroup{Files: []domain.ScannedFile{
		scannedFile("/music/CD1/01.flac", map[string]string{"track_number": "1", "disc_number": "1"}),
		scannedFile("/music/CD2/01.flac", map[string]string{"track_number": "1", "disc_number": "2"}),
		scannedFile("/music/CD2/02.flac", map[string]string{"track_number": "2"}), // missing tag → defaults to "1"
	}}
	candidate := domain.MusicReleaseCandidate{ReleaseMBID: testReleaseMBID}

	if err := importer.ImportRelease(t.Context(), candidate, group); err != nil {
		t.Fatalf("ImportRelease: %v", err)
	}

	want := []string{"1", "2", "1"}
	for i, w := range want {
		if got := items.saved[i].Metadata["disc_number"]; got != w {
			t.Errorf("item[%d] disc_number = %v, want %q", i, got, w)
		}
	}
}

func TestReleaseImporter_CallsTagWriterPerTrack(t *testing.T) {
	releases := newMockReleaseRepo(testReleaseMBID, stubRelease())
	tagWriter := &mockTagWriter{}
	importer := newImporter(releases, &mockItemRepo{}, &mockMediaFileRepo{}, &mockFileSystem{}, tagWriter)

	group := ports.ScannedFileGroup{Files: []domain.ScannedFile{
		scannedFile("/music/01.flac", map[string]string{"track_number": "1", "musicbrainz_track_id": "rec-1"}),
		scannedFile("/music/02.flac", map[string]string{"track_number": "2", "musicbrainz_track_id": "rec-2"}),
	}}
	candidate := domain.MusicReleaseCandidate{ReleaseMBID: testReleaseMBID}

	if err := importer.ImportRelease(t.Context(), candidate, group); err != nil {
		t.Fatalf("ImportRelease: %v", err)
	}

	if len(tagWriter.calls) != 2 {
		t.Fatalf("WriteIDs called %d times, want 2", len(tagWriter.calls))
	}
	if tagWriter.calls[0] != (tagWriteCall{"/music/01.flac", testReleaseMBID, "rec-1"}) {
		t.Errorf("call[0] = %+v", tagWriter.calls[0])
	}
	if tagWriter.calls[1] != (tagWriteCall{"/music/02.flac", testReleaseMBID, "rec-2"}) {
		t.Errorf("call[1] = %+v", tagWriter.calls[1])
	}
}

func TestReleaseImporter_TagWriterFailure_DoesNotAbortImport(t *testing.T) {
	releases := newMockReleaseRepo(testReleaseMBID, stubRelease())
	items := &mockItemRepo{}
	mediaFiles := &mockMediaFileRepo{}
	tagWriter := &mockTagWriter{err: errors.New("write failed")}
	importer := newImporter(releases, items, mediaFiles, &mockFileSystem{}, tagWriter)

	group := ports.ScannedFileGroup{Files: []domain.ScannedFile{
		scannedFile("/music/01.flac", map[string]string{"track_number": "1"}),
	}}
	candidate := domain.MusicReleaseCandidate{ReleaseMBID: testReleaseMBID}

	if err := importer.ImportRelease(t.Context(), candidate, group); err != nil {
		t.Fatalf("ImportRelease returned error despite tag-writer failure being non-fatal: %v", err)
	}
	if len(items.saved) != 1 {
		t.Errorf("items saved = %d, want 1", len(items.saved))
	}
	if len(mediaFiles.saved) != 1 {
		t.Errorf("media files saved = %d, want 1", len(mediaFiles.saved))
	}
	if releases.saved[0].Status != domain.ReleaseStatusImported {
		t.Errorf("release.Status = %q, want %q", releases.saved[0].Status, domain.ReleaseStatusImported)
	}
}
