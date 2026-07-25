package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	musicdomain "purser/internal/domain/music"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

type fakeMusicReleaseRepository struct {
	byID            map[string]*musicdomain.Release
	forceConflict   bool
	tracksByRelease map[string][]*domain.Item
}

func newFakeMusicReleaseRepository() *fakeMusicReleaseRepository {
	return &fakeMusicReleaseRepository{
		byID:            make(map[string]*musicdomain.Release),
		tracksByRelease: make(map[string][]*domain.Item),
	}
}

func (f *fakeMusicReleaseRepository) Create(_ context.Context, r *musicdomain.Release) error {
	if f.forceConflict {
		return ports.ErrConflict
	}
	if _, exists := f.byID[r.ID]; exists {
		return ports.ErrConflict
	}
	stored := *r
	f.byID[r.ID] = &stored
	return nil
}

func (f *fakeMusicReleaseRepository) Get(_ context.Context, id string) (*musicdomain.Release, error) {
	r, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *r
	return &stored, nil
}

func (f *fakeMusicReleaseRepository) GetByMBID(_ context.Context, mbid string) (*musicdomain.Release, error) {
	for _, r := range f.byID {
		if r.MBID == mbid {
			stored := *r
			return &stored, nil
		}
	}
	return nil, ports.ErrNotFound
}

func (f *fakeMusicReleaseRepository) GetByBarcode(_ context.Context, barcode string) (*musicdomain.Release, error) {
	for _, r := range f.byID {
		if r.Barcode == barcode {
			stored := *r
			return &stored, nil
		}
	}
	return nil, ports.ErrNotFound
}

func (f *fakeMusicReleaseRepository) Update(_ context.Context, r *musicdomain.Release) error {
	if _, exists := f.byID[r.ID]; !exists {
		return ports.ErrNotFound
	}
	stored := *r
	f.byID[r.ID] = &stored
	return nil
}

func (f *fakeMusicReleaseRepository) Delete(_ context.Context, id string) error {
	if _, exists := f.byID[id]; !exists {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeMusicReleaseRepository) List(_ context.Context, pageSize int, _ string) ([]*musicdomain.Release, string, error) {
	releases := make([]*musicdomain.Release, 0, len(f.byID))
	for _, r := range f.byID {
		stored := *r
		releases = append(releases, &stored)
	}
	if pageSize > 0 && len(releases) > pageSize {
		releases = releases[:pageSize]
	}
	return releases, "", nil
}

func (f *fakeMusicReleaseRepository) ListByGroup(_ context.Context, groupID string, pageSize int, _ string) ([]*musicdomain.Release, string, error) {
	var releases []*musicdomain.Release
	for _, r := range f.byID {
		if r.GroupID == groupID {
			stored := *r
			releases = append(releases, &stored)
		}
	}
	if pageSize > 0 && len(releases) > pageSize {
		releases = releases[:pageSize]
	}
	return releases, "", nil
}

func (f *fakeMusicReleaseRepository) ListByEntry(_ context.Context, libraryEntryID string, pageSize int, _ string) ([]*musicdomain.Release, string, error) {
	var releases []*musicdomain.Release
	for _, r := range f.byID {
		if r.LibraryEntryID == libraryEntryID {
			stored := *r
			releases = append(releases, &stored)
		}
	}
	if pageSize > 0 && len(releases) > pageSize {
		releases = releases[:pageSize]
	}
	return releases, "", nil
}

// ListTracksByRelease implements ports.MusicReleaseRepository. Mirrors the
// real repository's ErrNotFound-on-unknown-release behavior; tracksByRelease
// is seeded directly by tests, since this fake has no backing Item store of
// its own.
func (f *fakeMusicReleaseRepository) ListTracksByRelease(_ context.Context, releaseID string, pageSize int, _ string) ([]*domain.Item, string, error) {
	if _, ok := f.byID[releaseID]; !ok {
		return nil, "", ports.ErrNotFound
	}
	tracks := f.tracksByRelease[releaseID]
	if pageSize > 0 && len(tracks) > pageSize {
		tracks = tracks[:pageSize]
	}
	return tracks, "", nil
}

func validRelease(id string) *musicdomain.Release {
	return &musicdomain.Release{
		ID:             id,
		GroupID:        "group1",
		LibraryEntryID: "entry1",
		Title:          "Test Release",
		Status:         musicdomain.ReleaseStatusStub,
	}
}

func TestMusicReleaseService_Create(t *testing.T) {
	t.Run("valid release is persisted", func(t *testing.T) {
		repo := newFakeMusicReleaseRepository()
		svc := service.NewMusicReleaseService(repo)

		got, err := svc.Create(context.Background(), validRelease("r1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.ID == "" || got.ID == "r1" {
			t.Fatalf("Create returned ID %q, want a server-generated one", got.ID)
		}
	})

	t.Run("invalid release is rejected before touching the repository", func(t *testing.T) {
		repo := newFakeMusicReleaseRepository()
		svc := service.NewMusicReleaseService(repo)

		invalid := validRelease("r1")
		invalid.Title = ""

		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid release returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		repo := newFakeMusicReleaseRepository()
		repo.forceConflict = true
		svc := service.NewMusicReleaseService(repo)

		if _, err := svc.Create(context.Background(), validRelease("r1")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("Create returned %v, want ErrConflict", err)
		}
	})
}

func TestMusicReleaseService_Get(t *testing.T) {
	repo := newFakeMusicReleaseRepository()
	svc := service.NewMusicReleaseService(repo)

	created, err := svc.Create(context.Background(), validRelease("r1"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), created.ID); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing release returned %v, want ErrNotFound", err)
	}
}

func TestMusicReleaseService_GetByMBID(t *testing.T) {
	repo := newFakeMusicReleaseRepository()
	svc := service.NewMusicReleaseService(repo)

	rel := validRelease("r1")
	rel.MBID = "mbid-1"
	created, err := svc.Create(context.Background(), rel)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := svc.GetByMBID(context.Background(), "mbid-1")
	if err != nil {
		t.Fatalf("GetByMBID returned error: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("GetByMBID returned ID %q, want %q", got.ID, created.ID)
	}
	if _, err := svc.GetByMBID(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetByMBID on unknown MBID returned %v, want ErrNotFound", err)
	}
}

func TestMusicReleaseService_GetByBarcode(t *testing.T) {
	repo := newFakeMusicReleaseRepository()
	svc := service.NewMusicReleaseService(repo)

	rel := validRelease("r1")
	rel.Barcode = "barcode-1"
	created, err := svc.Create(context.Background(), rel)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := svc.GetByBarcode(context.Background(), "barcode-1")
	if err != nil {
		t.Fatalf("GetByBarcode returned error: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("GetByBarcode returned ID %q, want %q", got.ID, created.ID)
	}
	if _, err := svc.GetByBarcode(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetByBarcode on unknown barcode returned %v, want ErrNotFound", err)
	}
}

func TestMusicReleaseService_Update(t *testing.T) {
	t.Run("valid update is persisted", func(t *testing.T) {
		repo := newFakeMusicReleaseRepository()
		svc := service.NewMusicReleaseService(repo)

		created, err := svc.Create(context.Background(), validRelease("r1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		updated := validRelease(created.ID)
		updated.Title = "Updated"
		if _, err := svc.Update(context.Background(), updated); err != nil {
			t.Fatalf("Update returned error: %v", err)
		}

		got, err := svc.Get(context.Background(), created.ID)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.Title != "Updated" {
			t.Fatalf("Get after Update returned Title %q, want %q", got.Title, "Updated")
		}
	})

	t.Run("invalid release is rejected", func(t *testing.T) {
		repo := newFakeMusicReleaseRepository()
		svc := service.NewMusicReleaseService(repo)

		if _, err := svc.Create(context.Background(), validRelease("r1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		invalid := validRelease("r1")
		invalid.Title = ""
		var verr *domain.ValidationError
		if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Update with invalid release returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("update of a missing release returns ErrNotFound", func(t *testing.T) {
		repo := newFakeMusicReleaseRepository()
		svc := service.NewMusicReleaseService(repo)

		if _, err := svc.Update(context.Background(), validRelease("missing")); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Update on missing release returned %v, want ErrNotFound", err)
		}
	})
}

func TestMusicReleaseService_Delete(t *testing.T) {
	repo := newFakeMusicReleaseRepository()
	svc := service.NewMusicReleaseService(repo)

	created, err := svc.Create(context.Background(), validRelease("r1"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), created.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func TestMusicReleaseService_List(t *testing.T) {
	repo := newFakeMusicReleaseRepository()
	svc := service.NewMusicReleaseService(repo)

	for _, id := range []string{"r1", "r2"} {
		if _, err := svc.Create(context.Background(), validRelease(id)); err != nil {
			t.Fatalf("Create(%q) returned error: %v", id, err)
		}
	}

	releases, _, err := svc.List(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(releases) != 2 {
		t.Fatalf("List returned %d releases, want 2", len(releases))
	}
}

func TestMusicReleaseService_ListByGroup(t *testing.T) {
	repo := newFakeMusicReleaseRepository()
	svc := service.NewMusicReleaseService(repo)

	relA := validRelease("r1")
	relA.GroupID = "groupA"
	relB := validRelease("r2")
	relB.GroupID = "groupB"
	if _, err := svc.Create(context.Background(), relA); err != nil {
		t.Fatalf("Create(r1) returned error: %v", err)
	}
	if _, err := svc.Create(context.Background(), relB); err != nil {
		t.Fatalf("Create(r2) returned error: %v", err)
	}

	releases, _, err := svc.ListByGroup(context.Background(), "groupA", 10, "")
	if err != nil {
		t.Fatalf("ListByGroup returned error: %v", err)
	}
	if len(releases) != 1 || releases[0].GroupID != "groupA" {
		t.Fatalf("ListByGroup(groupA) returned %v, want exactly one release in groupA", releases)
	}
}

func TestMusicReleaseService_ListByEntry(t *testing.T) {
	repo := newFakeMusicReleaseRepository()
	svc := service.NewMusicReleaseService(repo)

	relA := validRelease("r1")
	relA.LibraryEntryID = "entryA"
	relB := validRelease("r2")
	relB.LibraryEntryID = "entryB"
	if _, err := svc.Create(context.Background(), relA); err != nil {
		t.Fatalf("Create(r1) returned error: %v", err)
	}
	if _, err := svc.Create(context.Background(), relB); err != nil {
		t.Fatalf("Create(r2) returned error: %v", err)
	}

	releases, _, err := svc.ListByEntry(context.Background(), "entryA", 10, "")
	if err != nil {
		t.Fatalf("ListByEntry returned error: %v", err)
	}
	if len(releases) != 1 || releases[0].LibraryEntryID != "entryA" {
		t.Fatalf("ListByEntry(entryA) returned %v, want exactly one release in entryA", releases)
	}
}

func TestMusicReleaseService_ListTracksByRelease(t *testing.T) {
	repo := newFakeMusicReleaseRepository()
	svc := service.NewMusicReleaseService(repo)

	created, err := svc.Create(context.Background(), validRelease("r1"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	track := &domain.Item{ID: "item1", ContentType: domain.ContentTypeMusic, LibraryEntryID: "entry1", GroupID: "group1", Title: "Track 1", Status: domain.ItemStatusImported}
	repo.tracksByRelease[created.ID] = []*domain.Item{track}

	tracks, _, err := svc.ListTracksByRelease(context.Background(), created.ID, 10, "")
	if err != nil {
		t.Fatalf("ListTracksByRelease returned error: %v", err)
	}
	if len(tracks) != 1 || tracks[0].ID != "item1" {
		t.Fatalf("ListTracksByRelease returned %v, want exactly track item1", tracks)
	}

	if _, _, err := svc.ListTracksByRelease(context.Background(), "missing", 10, ""); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("ListTracksByRelease on missing release returned %v, want ErrNotFound", err)
	}
}
