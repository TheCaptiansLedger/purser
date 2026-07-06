package identifier_test

import (
	"context"
	"errors"
	"purser/internal/adapters/identifier"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// ── mock ─────────────────────────────────────────────────────────────────────

type stubMusicScanGroupRepo struct {
	entries []*domain.MusicScanGroup
	saveFn  func(*domain.MusicScanGroup) error
	listFn  func(domain.UnmatchedStatus) ([]*domain.MusicScanGroup, error)
}

func (r *stubMusicScanGroupRepo) Get(_ context.Context, id string) (*domain.MusicScanGroup, error) {
	for _, e := range r.entries {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, errNotFoundStub
}

var errNotFoundStub = errors.New("not found")

func (r *stubMusicScanGroupRepo) List(_ context.Context, status domain.UnmatchedStatus) ([]*domain.MusicScanGroup, error) {
	if r.listFn != nil {
		return r.listFn(status)
	}
	var out []*domain.MusicScanGroup
	for _, e := range r.entries {
		if e.Status == status {
			out = append(out, e)
		}
	}
	return out, nil
}

func (r *stubMusicScanGroupRepo) Save(_ context.Context, g *domain.MusicScanGroup) error {
	if r.saveFn != nil {
		return r.saveFn(g)
	}
	for i, e := range r.entries {
		if e.ID == g.ID {
			r.entries[i] = g
			return nil
		}
	}
	r.entries = append(r.entries, g)
	return nil
}

func (r *stubMusicScanGroupRepo) Delete(_ context.Context, id string) error {
	for i, e := range r.entries {
		if e.ID == id {
			r.entries = append(r.entries[:i], r.entries[i+1:]...)
			return nil
		}
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func files(paths ...string) []domain.ScannedFile {
	out := make([]domain.ScannedFile, len(paths))
	for i, p := range paths {
		out[i] = domain.ScannedFile{Path: p}
	}
	return out
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestMusicGroupQueueWriter_ContentTypes(t *testing.T) {
	w := identifier.NewMusicGroupQueueWriter(&stubMusicScanGroupRepo{})
	for _, ct := range w.ContentTypes() {
		if ct == domain.ContentTypeMusic {
			return
		}
	}
	t.Fatal("expected ContentTypeMusic in ContentTypes")
}

func TestMusicGroupQueueWriter_SingleDisc_CreatesEntry(t *testing.T) {
	repo := &stubMusicScanGroupRepo{}
	w := identifier.NewMusicGroupQueueWriter(repo)

	group := ports.ScannedFileGroup{
		RootPath: "/music/Hi Infidelity",
		Files: files(
			"/music/Hi Infidelity/01.flac",
			"/music/Hi Infidelity/02.flac",
			"/music/Hi Infidelity/10.flac",
		),
	}

	if err := w.Identify(context.Background(), group); err != nil {
		t.Fatalf("Identify: %v", err)
	}

	if len(repo.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(repo.entries))
	}
	got := repo.entries[0]
	if got.FolderPath != "/music/Hi Infidelity" {
		t.Errorf("FolderPath: got %q, want %q", got.FolderPath, "/music/Hi Infidelity")
	}
	if got.TotalTracks != 3 {
		t.Errorf("TotalTracks: got %d, want 3", got.TotalTracks)
	}
	if got.TotalDiscs != 1 {
		t.Errorf("TotalDiscs: got %d, want 1", got.TotalDiscs)
	}
	if got.Status != domain.UnmatchedPending {
		t.Errorf("Status: got %v, want pending", got.Status)
	}
	if got.ID == "" {
		t.Error("ID must not be empty")
	}
	if got.DiscoveredAt.IsZero() {
		t.Error("DiscoveredAt must not be zero")
	}
}

func TestMusicGroupQueueWriter_MultiDisc_CountsDiscs(t *testing.T) {
	repo := &stubMusicScanGroupRepo{}
	w := identifier.NewMusicGroupQueueWriter(repo)

	group := ports.ScannedFileGroup{
		RootPath: "/music/The Wall",
		Files: files(
			"/music/The Wall/CD1/01.flac",
			"/music/The Wall/CD1/02.flac",
			"/music/The Wall/CD2/01.flac",
			"/music/The Wall/CD2/02.flac",
		),
	}

	if err := w.Identify(context.Background(), group); err != nil {
		t.Fatalf("Identify: %v", err)
	}

	got := repo.entries[0]
	if got.TotalTracks != 4 {
		t.Errorf("TotalTracks: got %d, want 4", got.TotalTracks)
	}
	if got.TotalDiscs != 2 {
		t.Errorf("TotalDiscs: got %d, want 2", got.TotalDiscs)
	}
}

func TestMusicGroupQueueWriter_Idempotent_UpdatesExisting(t *testing.T) {
	existing := &domain.MusicScanGroup{
		ID:          "existing-id",
		FolderPath:  "/music/Hi Infidelity",
		TotalTracks: 8,
		TotalDiscs:  1,
		Status:      domain.UnmatchedPending,
	}
	repo := &stubMusicScanGroupRepo{entries: []*domain.MusicScanGroup{existing}}
	w := identifier.NewMusicGroupQueueWriter(repo)

	group := ports.ScannedFileGroup{
		RootPath: "/music/Hi Infidelity",
		Files: files(
			"/music/Hi Infidelity/01.flac",
			"/music/Hi Infidelity/02.flac",
			"/music/Hi Infidelity/10.flac",
		),
	}

	if err := w.Identify(context.Background(), group); err != nil {
		t.Fatalf("Identify: %v", err)
	}

	// Must not create a second entry.
	if len(repo.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d (must not duplicate)", len(repo.entries))
	}
	got := repo.entries[0]
	if got.ID != "existing-id" {
		t.Errorf("ID: got %q, want %q", got.ID, "existing-id")
	}
	if got.TotalTracks != 3 {
		t.Errorf("TotalTracks: got %d, want 3 (updated)", got.TotalTracks)
	}
}

func TestMusicGroupQueueWriter_ListError_Propagates(t *testing.T) {
	repo := &stubMusicScanGroupRepo{
		listFn: func(domain.UnmatchedStatus) ([]*domain.MusicScanGroup, error) {
			return nil, errors.New("storage failure")
		},
	}
	w := identifier.NewMusicGroupQueueWriter(repo)
	group := ports.ScannedFileGroup{
		RootPath: "/music/Album",
		Files:    files("/music/Album/01.flac"),
	}
	if err := w.Identify(context.Background(), group); err == nil {
		t.Fatal("expected error from List, got nil")
	}
}

func TestMusicGroupQueueWriter_SaveError_Propagates(t *testing.T) {
	repo := &stubMusicScanGroupRepo{
		saveFn: func(*domain.MusicScanGroup) error {
			return errors.New("disk full")
		},
	}
	w := identifier.NewMusicGroupQueueWriter(repo)
	group := ports.ScannedFileGroup{
		RootPath: "/music/Album",
		Files:    files("/music/Album/01.flac"),
	}
	if err := w.Identify(context.Background(), group); err == nil {
		t.Fatal("expected error from Save, got nil")
	}
}
