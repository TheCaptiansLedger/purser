package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// deletionFakeItemRepositoryForMusic is the minimal ports.ItemRepository
// fake MusicReleaseDeletionService needs — it only ever calls items.Update,
// since track lookup goes through MusicReleaseRepository.ListTracksByRelease
// per docs/adr/0021-music-domain-model.md's "Track ↔ Release linkage"
// section, not through this port's List.
type deletionFakeItemRepositoryForMusic struct {
	byID      map[string]*domain.Item
	updateErr error
}

func (f *deletionFakeItemRepositoryForMusic) Create(_ context.Context, i *domain.Item) error {
	f.byID[i.ID] = i
	return nil
}

func (f *deletionFakeItemRepositoryForMusic) Get(_ context.Context, id string) (*domain.Item, error) {
	i, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return i, nil
}

func (f *deletionFakeItemRepositoryForMusic) Update(_ context.Context, i *domain.Item) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.byID[i.ID] = i
	return nil
}

func (f *deletionFakeItemRepositoryForMusic) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

func (f *deletionFakeItemRepositoryForMusic) List(_ context.Context, _, _, _ string, _ int, _ string) ([]*domain.Item, string, error) {
	return nil, "", nil
}

func (f *deletionFakeItemRepositoryForMusic) DeleteBatch(_ context.Context, ids []string) error {
	for _, id := range ids {
		delete(f.byID, id)
	}
	return nil
}

func newMusicReleaseDeletionFixture() (*service.MusicReleaseDeletionService, *fakeMusicReleaseRepository, *deletionFakeItemRepositoryForMusic) {
	releases := newFakeMusicReleaseRepository()
	rel := validRelease("r1")
	if err := releases.Create(context.Background(), rel); err != nil {
		panic(err)
	}

	track1 := &domain.Item{ID: "i1", ContentType: domain.ContentTypeMusic, LibraryEntryID: "entry1", GroupID: "group1", Title: "Track 1", Status: domain.ItemStatusImported, Metadata: map[string]any{"release_id": "r1", "disc_number": 1}}
	track2 := &domain.Item{ID: "i2", ContentType: domain.ContentTypeMusic, LibraryEntryID: "entry1", GroupID: "group1", Title: "Track 2", Status: domain.ItemStatusImported, Metadata: map[string]any{"release_id": "r1"}}
	releases.tracksByRelease["r1"] = []*domain.Item{track1, track2}

	items := &deletionFakeItemRepositoryForMusic{byID: map[string]*domain.Item{"i1": track1, "i2": track2}}

	svc := service.NewMusicReleaseDeletionService(releases, items)
	return svc, releases, items
}

func TestMusicReleaseDeletionService_GetDeletionImpact(t *testing.T) {
	svc, _, _ := newMusicReleaseDeletionFixture()

	impact, err := svc.GetDeletionImpact(context.Background(), "r1")
	if err != nil {
		t.Fatalf("GetDeletionImpact returned error: %v", err)
	}
	if len(impact.Impacts) != 1 {
		t.Fatalf("GetDeletionImpact returned %d rows, want 1", len(impact.Impacts))
	}
	row := impact.Impacts[0]
	if row.Kind != "item" || row.Label != "Tracks" || row.Count != 2 {
		t.Fatalf("GetDeletionImpact returned row %+v, want {Kind:item Label:Tracks Count:2}", row)
	}
	if row.Blocking {
		t.Fatal("GetDeletionImpact row marked Blocking, want false — Release never blocks a delete")
	}

	if _, err := svc.GetDeletionImpact(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetDeletionImpact on missing release returned %v, want ErrNotFound", err)
	}
}

func TestMusicReleaseDeletionService_Delete_DetachesTracksRatherThanDeletingThem(t *testing.T) {
	svc, releases, items := newMusicReleaseDeletionFixture()

	if err := svc.Delete(context.Background(), "r1", false); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if _, err := releases.Get(context.Background(), "r1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("Delete did not remove the Release itself")
	}

	i1, err := items.Get(context.Background(), "i1")
	if err != nil {
		t.Fatalf("Delete removed Item i1 entirely, want it detached but intact: %v", err)
	}
	if _, ok := i1.Metadata["release_id"]; ok {
		t.Fatalf("Delete left Item i1 Metadata[release_id] = %v, want the key cleared", i1.Metadata["release_id"])
	}
	if i1.Metadata["disc_number"] != 1 {
		t.Fatalf("Delete changed unrelated Metadata key disc_number to %v, want it untouched", i1.Metadata["disc_number"])
	}

	i2, err := items.Get(context.Background(), "i2")
	if err != nil {
		t.Fatalf("Delete removed Item i2 entirely, want it detached but intact: %v", err)
	}
	if _, ok := i2.Metadata["release_id"]; ok {
		t.Fatalf("Delete left Item i2 Metadata[release_id] = %v, want the key cleared", i2.Metadata["release_id"])
	}
}

func TestMusicReleaseDeletionService_DeleteMissing(t *testing.T) {
	svc, _, _ := newMusicReleaseDeletionFixture()

	if err := svc.Delete(context.Background(), "missing", false); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete on missing release returned %v, want ErrNotFound", err)
	}
}

func TestMusicReleaseDeletionService_Delete_CascadeHasNoEffect(t *testing.T) {
	svc, releases, items := newMusicReleaseDeletionFixture()

	if err := svc.Delete(context.Background(), "r1", true); err != nil {
		t.Fatalf("Delete(cascade=true) returned error: %v", err)
	}
	if _, err := releases.Get(context.Background(), "r1"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("Delete did not remove the Release itself")
	}
	if _, err := items.Get(context.Background(), "i1"); err != nil {
		t.Fatalf("Delete(cascade=true) removed the track, want Unlink-only behavior regardless of cascade: %v", err)
	}
}

func TestMusicReleaseDeletionService_Delete_PropagatesPortErrors(t *testing.T) {
	t.Run("items Update error propagates", func(t *testing.T) {
		svc, _, items := newMusicReleaseDeletionFixture()
		items.updateErr = errBoom
		if err := svc.Delete(context.Background(), "r1", false); !errors.Is(err, errBoom) {
			t.Fatalf("Delete returned %v, want errBoom", err)
		}
	})

	t.Run("GetDeletionImpact propagates a not-found", func(t *testing.T) {
		svc, releases, _ := newMusicReleaseDeletionFixture()
		delete(releases.byID, "r1")
		if _, err := svc.GetDeletionImpact(context.Background(), "r1"); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("GetDeletionImpact returned %v, want ErrNotFound", err)
		}
	})
}
