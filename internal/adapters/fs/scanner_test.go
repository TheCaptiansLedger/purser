package fs_test

import (
	"context"
	"purser/internal/adapters/fs"
	"purser/internal/app/errs"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// mockFS implements ports.FileSystem for scanner tests using an in-memory file list.
type mockFS struct {
	files []ports.FileInfo
}

func (m *mockFS) Walk(_ context.Context, _ string, fn func(ports.FileInfo) error) error {
	for _, f := range m.files {
		if err := fn(f); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockFS) Stat(_ context.Context, _ string) (*ports.FileInfo, error) {
	return nil, errs.ErrNotFound
}
func (m *mockFS) Move(_ context.Context, _, _ string) error          { return nil }
func (m *mockFS) OSHash(_ context.Context, _ string) (string, error) { return "", nil }
func (m *mockFS) SHA1(_ context.Context, _ string) (string, error)   { return "", nil }

// mockMediaFileRepo implements ports.MediaFileRepository for scanner tests.
type mockMediaFileRepo struct {
	byPath map[string]*domain.MediaFile
}

func newMockRepo(files ...*domain.MediaFile) *mockMediaFileRepo {
	m := &mockMediaFileRepo{byPath: make(map[string]*domain.MediaFile, len(files))}
	for _, f := range files {
		m.byPath[f.Path] = f
	}
	return m
}

func (m *mockMediaFileRepo) GetByPath(_ context.Context, path string) (*domain.MediaFile, error) {
	if f, ok := m.byPath[path]; ok {
		return f, nil
	}
	return nil, errs.ErrNotFound
}

func (m *mockMediaFileRepo) GetByItemID(_ context.Context, _ string) (*domain.MediaFile, error) {
	return nil, errs.ErrNotFound
}

func (m *mockMediaFileRepo) GetByOSHash(_ context.Context, _ string) (*domain.MediaFile, error) {
	return nil, errs.ErrNotFound
}
func (m *mockMediaFileRepo) Save(_ context.Context, _ *domain.MediaFile) error { return nil }
func (m *mockMediaFileRepo) Delete(_ context.Context, _ string) error          { return nil }

func drain(t *testing.T, ch <-chan domain.ScannedFile) []domain.ScannedFile {
	t.Helper()
	var out []domain.ScannedFile
	for f := range ch {
		out = append(out, f)
	}
	return out
}

func TestScanner_OnlyMediaExtensions(t *testing.T) {
	mfs := &mockFS{files: []ports.FileInfo{
		{Path: "/data/song.flac", Size: 1000},
		{Path: "/data/cover.jpg", Size: 500},
		{Path: "/data/notes.txt", Size: 100},
	}}
	s := fs.NewScanner(mfs, newMockRepo())
	ch, err := s.Scan(context.Background(), []string{"/data"}, ports.ScanFilter{})
	if err != nil {
		t.Fatal(err)
	}
	got := drain(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1 (only .flac)", len(got))
	}
	if got[0].Path != "/data/song.flac" {
		t.Errorf("path = %q, want /data/song.flac", got[0].Path)
	}
	if got[0].ContentType != domain.ContentTypeMusic {
		t.Errorf("content type = %q, want music", got[0].ContentType)
	}
}

func TestScanner_SkipsDirectories(t *testing.T) {
	mfs := &mockFS{files: []ports.FileInfo{
		{Path: "/data", IsDir: true},
		{Path: "/data/sub", IsDir: true},
		{Path: "/data/sub/track.mp3", Size: 2000},
	}}
	s := fs.NewScanner(mfs, newMockRepo())
	ch, _ := s.Scan(context.Background(), []string{"/data"}, ports.ScanFilter{})
	got := drain(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1 (dirs skipped)", len(got))
	}
}

func TestScanner_SkipsKnownFileSameSize(t *testing.T) {
	mfs := &mockFS{files: []ports.FileInfo{
		{Path: "/data/movie.mkv", Size: 2000},
	}}
	known := &domain.MediaFile{ID: "1", Path: "/data/movie.mkv", Size: 2000}
	s := fs.NewScanner(mfs, newMockRepo(known))
	ch, _ := s.Scan(context.Background(), []string{"/data"}, ports.ScanFilter{})
	got := drain(t, ch)
	if len(got) != 0 {
		t.Fatalf("got %d results, want 0 (known same-size file should be skipped)", len(got))
	}
}

func TestScanner_EmitsReplacedFile(t *testing.T) {
	mfs := &mockFS{files: []ports.FileInfo{
		{Path: "/data/movie.mkv", Size: 5000},
	}}
	// Recorded size is smaller — the file was replaced.
	known := &domain.MediaFile{ID: "1", Path: "/data/movie.mkv", Size: 2000}
	s := fs.NewScanner(mfs, newMockRepo(known))
	ch, _ := s.Scan(context.Background(), []string{"/data"}, ports.ScanFilter{})
	got := drain(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1 (replaced file must be re-emitted)", len(got))
	}
	if got[0].Size != 5000 {
		t.Errorf("size = %d, want 5000", got[0].Size)
	}
}

func TestScanner_ContentTypeFilter(t *testing.T) {
	mfs := &mockFS{files: []ports.FileInfo{
		{Path: "/data/song.flac", Size: 1000},
		{Path: "/data/movie.mkv", Size: 2000},
		{Path: "/data/book.epub", Size: 500},
	}}
	s := fs.NewScanner(mfs, newMockRepo())
	ch, _ := s.Scan(context.Background(), []string{"/data"}, ports.ScanFilter{
		ContentTypes: []domain.ContentType{domain.ContentTypeMusic},
	})
	got := drain(t, ch)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1 (music filter)", len(got))
	}
	if got[0].ContentType != domain.ContentTypeMusic {
		t.Errorf("content type = %q, want music", got[0].ContentType)
	}
}

func TestScanner_MultipleRoots(t *testing.T) {
	mfs := &mockFS{files: []ports.FileInfo{
		{Path: "/disk1/a.flac", Size: 100},
		{Path: "/disk2/b.mp3", Size: 200},
	}}
	s := fs.NewScanner(mfs, newMockRepo())
	ch, _ := s.Scan(context.Background(), []string{"/disk1", "/disk2"}, ports.ScanFilter{})
	got := drain(t, ch)
	// mockFS always walks all files regardless of root; 2 roots → 4 total,
	// but duplicates share path so both are emitted (mockRepo returns not-found).
	if len(got) == 0 {
		t.Fatal("expected at least one result for multiple roots")
	}
}
