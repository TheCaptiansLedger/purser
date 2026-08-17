package service_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"sort"
	"testing"
)

type fakeImageRepository struct {
	byID map[string]*domain.Image
	// forceConflict makes the next Create return ports.ErrConflict without
	// touching byID — with server-generated IDs (see
	// docs/adr/0020-server-generated-kernel-entity-ids.md), two Creates
	// can no longer be forced to collide by reusing a literal ID.
	forceConflict bool
}

func newFakeImageRepository() *fakeImageRepository {
	return &fakeImageRepository{byID: make(map[string]*domain.Image)}
}

func (f *fakeImageRepository) Create(_ context.Context, img *domain.Image) error {
	if f.forceConflict {
		return ports.ErrConflict
	}
	if _, exists := f.byID[img.ID]; exists {
		return ports.ErrConflict
	}
	stored := *img
	f.byID[img.ID] = &stored
	return nil
}

func (f *fakeImageRepository) Get(_ context.Context, id string) (*domain.Image, error) {
	img, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *img
	return &stored, nil
}

func (f *fakeImageRepository) Update(_ context.Context, img *domain.Image) error {
	if _, ok := f.byID[img.ID]; !ok {
		return ports.ErrNotFound
	}
	stored := *img
	f.byID[img.ID] = &stored
	return nil
}

func (f *fakeImageRepository) Delete(_ context.Context, id string) error {
	if _, ok := f.byID[id]; !ok {
		return ports.ErrNotFound
	}
	delete(f.byID, id)
	return nil
}

// List mimics store.FilteredRepository.List's real contract — ID-ascending
// order, "token = last returned ID" cursor, ownerType/ownerID as
// independent optional filters — rather than ignoring pagination/filter
// args outright, so ImageService.List's own ordering/pagination logic (it
// re-sorts and re-paginates an owner-scoped fetch, see image.go) is
// exercised against something real instead of a stub that always hands
// back everything in one call.
func (f *fakeImageRepository) List(_ context.Context, ownerType, ownerID string, pageSize int, pageToken string) ([]*domain.Image, string, error) {
	ids := make([]string, 0, len(f.byID))
	for id := range f.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	if pageSize <= 0 {
		pageSize = 50
	}

	var matched []*domain.Image
	for _, id := range ids {
		if id <= pageToken {
			continue
		}
		img := f.byID[id]
		if ownerType != "" && img.OwnerType != ownerType {
			continue
		}
		if ownerID != "" && img.OwnerID != ownerID {
			continue
		}
		matched = append(matched, img)
	}

	next := ""
	if len(matched) > pageSize {
		matched = matched[:pageSize]
		next = matched[len(matched)-1].ID
	}

	out := make([]*domain.Image, len(matched))
	for i, img := range matched {
		stored := *img
		out[i] = &stored
	}
	return out, next, nil
}

// fakeImageSelectionRepository mimics the real adapter's contract closely
// enough to exercise ImageService's own get-or-create-or-update logic
// (setSelection) — Create conflicts on an occupied slot, Update 404s on
// an empty one, same as internal/adapters/store/imageselection.
type fakeImageSelectionRepository struct {
	bySlot map[string]*domain.ImageSelection
}

func newFakeImageSelectionRepository() *fakeImageSelectionRepository {
	return &fakeImageSelectionRepository{bySlot: make(map[string]*domain.ImageSelection)}
}

func selectionKey(ownerType, ownerID string, imageType domain.ImageType) string {
	return ownerType + "/" + ownerID + "/" + string(imageType)
}

func (f *fakeImageSelectionRepository) Create(_ context.Context, sel *domain.ImageSelection) error {
	key := selectionKey(sel.OwnerType, sel.OwnerID, sel.ImageType)
	if _, exists := f.bySlot[key]; exists {
		return ports.ErrConflict
	}
	stored := *sel
	f.bySlot[key] = &stored
	return nil
}

func (f *fakeImageSelectionRepository) Get(_ context.Context, ownerType, ownerID string, imageType domain.ImageType) (*domain.ImageSelection, error) {
	sel, ok := f.bySlot[selectionKey(ownerType, ownerID, imageType)]
	if !ok {
		return nil, ports.ErrNotFound
	}
	stored := *sel
	return &stored, nil
}

func (f *fakeImageSelectionRepository) Update(_ context.Context, sel *domain.ImageSelection) error {
	key := selectionKey(sel.OwnerType, sel.OwnerID, sel.ImageType)
	if _, ok := f.bySlot[key]; !ok {
		return ports.ErrNotFound
	}
	stored := *sel
	f.bySlot[key] = &stored
	return nil
}

func (f *fakeImageSelectionRepository) Delete(_ context.Context, ownerType, ownerID string, imageType domain.ImageType) error {
	key := selectionKey(ownerType, ownerID, imageType)
	if _, ok := f.bySlot[key]; !ok {
		return ports.ErrNotFound
	}
	delete(f.bySlot, key)
	return nil
}

// fakeImageStore itself is declared in image_blob_test.go, shared by both
// files. ImageService.Delete only ever calls Delete on it here (Put/Get
// belong to the blob-attach flow ImageBlobService owns).
func newFakeImageStore() *fakeImageStore { return &fakeImageStore{} }

func validImage(id string) *domain.Image {
	return &domain.Image{ID: id, OwnerType: "person", OwnerID: "p1", ImageType: domain.ImageTypePoster, URL: "https://example.com/i.jpg"}
}

func newTestImageService() (*service.ImageService, *fakeImageRepository, *fakeImageSelectionRepository, *fakeImageStore) {
	repo := newFakeImageRepository()
	selections := newFakeImageSelectionRepository()
	store := newFakeImageStore()
	return service.NewImageService(repo, selections, store), repo, selections, store
}

func TestImageService_Create(t *testing.T) {
	t.Run("valid image is persisted", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		got, err := svc.Create(context.Background(), validImage("i1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if got.ID == "" || got.ID == "i1" {
			t.Fatalf("Create returned ID %q, want a server-generated one", got.ID)
		}
	})

	t.Run("invalid image is rejected before touching the repository", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		invalid := validImage("i1")
		invalid.URL = ""
		var verr *domain.ValidationError
		if _, err := svc.Create(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Create with invalid image returned %v, want *domain.ValidationError", err)
		}
	})

	t.Run("a repository conflict is propagated", func(t *testing.T) {
		svc, repo, _, _ := newTestImageService()
		repo.forceConflict = true

		if _, err := svc.Create(context.Background(), validImage("i1")); !errors.Is(err, ports.ErrConflict) {
			t.Fatalf("Create returned %v, want ErrConflict", err)
		}
	})

	t.Run("create does not select the image — attaching and selecting are separate calls", func(t *testing.T) {
		svc, _, selections, _ := newTestImageService()

		created, err := svc.Create(context.Background(), validImage("i1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if _, err := selections.Get(context.Background(), created.OwnerType, created.OwnerID, created.ImageType); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("selections.Get after a bare Create returned %v, want ErrNotFound", err)
		}
	})
}

func TestImageService_Get(t *testing.T) {
	svc, _, _, _ := newTestImageService()

	created, err := svc.Create(context.Background(), validImage("i1"))
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), created.ID); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on missing image returned %v, want ErrNotFound", err)
	}
}

func TestImageService_Update(t *testing.T) {
	t.Run("valid update is persisted", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		created, err := svc.Create(context.Background(), validImage("i1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		updated := validImage(created.ID)
		updated.Priority = 5
		if _, err := svc.Update(context.Background(), updated); err != nil {
			t.Fatalf("Update returned error: %v", err)
		}

		got, err := svc.Get(context.Background(), created.ID)
		if err != nil {
			t.Fatalf("Get returned error: %v", err)
		}
		if got.Priority != 5 {
			t.Fatalf("Get after Update returned Priority %d, want 5", got.Priority)
		}
	})

	t.Run("update of a missing image returns ErrNotFound", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		if _, err := svc.Update(context.Background(), validImage("missing")); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Update on missing image returned %v, want ErrNotFound", err)
		}
	})

	t.Run("invalid image is rejected", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		if _, err := svc.Create(context.Background(), validImage("i1")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		invalid := validImage("i1")
		invalid.URL = ""
		var verr *domain.ValidationError
		if _, err := svc.Update(context.Background(), invalid); !errors.As(err, &verr) {
			t.Fatalf("Update with invalid image returned %v, want *domain.ValidationError", err)
		}
	})
}

func TestImageService_Delete(t *testing.T) {
	t.Run("removes the row and frees the underlying blob", func(t *testing.T) {
		svc, _, _, store := newTestImageService()

		created, err := svc.Create(context.Background(), validImage("i1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if err := svc.Delete(context.Background(), created.ID); err != nil {
			t.Fatalf("Delete returned error: %v", err)
		}
		if _, err := svc.Get(context.Background(), created.ID); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
		}
		if len(store.deleted) != 1 || store.deleted[0] != created.URL {
			t.Fatalf("ImageStore.Delete calls = %v, want exactly [%q]", store.deleted, created.URL)
		}
	})

	t.Run("delete of a missing image returns ErrNotFound", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		if err := svc.Delete(context.Background(), "missing"); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Delete on missing image returned %v, want ErrNotFound", err)
		}
	})

	t.Run("a blob cleanup failure does not roll back the already-successful row delete", func(t *testing.T) {
		svc, _, _, store := newTestImageService()
		store.deleteErr = errors.New("disk unavailable")

		created, err := svc.Create(context.Background(), validImage("i1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if err := svc.Delete(context.Background(), created.ID); err != nil {
			t.Fatalf("Delete returned error: %v, want nil — a blob-cleanup failure is an accepted, bounded risk", err)
		}
		if _, err := svc.Get(context.Background(), created.ID); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
		}
	})

	t.Run("deleting the selected image reassigns the slot to the newest remaining image", func(t *testing.T) {
		svc, _, selections, _ := newTestImageService()

		older, err := svc.Create(context.Background(), validImage(""))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		newer, err := svc.Create(context.Background(), validImage(""))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if _, err := svc.Select(context.Background(), "person", "p1", domain.ImageTypePoster, older.ID); err != nil {
			t.Fatalf("Select returned error: %v", err)
		}

		if err := svc.Delete(context.Background(), older.ID); err != nil {
			t.Fatalf("Delete returned error: %v", err)
		}

		sel, err := selections.Get(context.Background(), "person", "p1", domain.ImageTypePoster)
		if err != nil {
			t.Fatalf("selections.Get returned error: %v", err)
		}
		if sel.ImageID != newer.ID {
			t.Fatalf("selection after deleting the current image points at %q, want the remaining image %q", sel.ImageID, newer.ID)
		}
	})

	t.Run("deleting the last remaining selected image clears the slot", func(t *testing.T) {
		svc, _, selections, _ := newTestImageService()

		only, err := svc.Create(context.Background(), validImage(""))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if _, err := svc.Select(context.Background(), "person", "p1", domain.ImageTypePoster, only.ID); err != nil {
			t.Fatalf("Select returned error: %v", err)
		}

		if err := svc.Delete(context.Background(), only.ID); err != nil {
			t.Fatalf("Delete returned error: %v", err)
		}

		if _, err := selections.Get(context.Background(), "person", "p1", domain.ImageTypePoster); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("selections.Get after deleting the only image returned %v, want ErrNotFound", err)
		}
	})

	t.Run("deleting an image that isn't the selected one leaves the selection untouched", func(t *testing.T) {
		svc, _, selections, _ := newTestImageService()

		selected, err := svc.Create(context.Background(), validImage(""))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		other, err := svc.Create(context.Background(), validImage(""))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if _, err := svc.Select(context.Background(), "person", "p1", domain.ImageTypePoster, selected.ID); err != nil {
			t.Fatalf("Select returned error: %v", err)
		}

		if err := svc.Delete(context.Background(), other.ID); err != nil {
			t.Fatalf("Delete returned error: %v", err)
		}

		sel, err := selections.Get(context.Background(), "person", "p1", domain.ImageTypePoster)
		if err != nil {
			t.Fatalf("selections.Get returned error: %v", err)
		}
		if sel.ImageID != selected.ID {
			t.Fatalf("selection changed to %q after deleting an unrelated image, want unchanged %q", sel.ImageID, selected.ID)
		}
	})
}

func TestImageService_Select(t *testing.T) {
	t.Run("selecting an image creates the slot's selection", func(t *testing.T) {
		svc, _, selections, _ := newTestImageService()

		img, err := svc.Create(context.Background(), validImage("i1"))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if _, err := svc.Select(context.Background(), "person", "p1", domain.ImageTypePoster, img.ID); err != nil {
			t.Fatalf("Select returned error: %v", err)
		}

		sel, err := selections.Get(context.Background(), "person", "p1", domain.ImageTypePoster)
		if err != nil {
			t.Fatalf("selections.Get returned error: %v", err)
		}
		if sel.ImageID != img.ID {
			t.Fatalf("selection ImageID = %q, want %q", sel.ImageID, img.ID)
		}
	})

	t.Run("selecting a second image overwrites which one the slot points at", func(t *testing.T) {
		svc, _, selections, _ := newTestImageService()

		first, err := svc.Create(context.Background(), validImage(""))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		second, err := svc.Create(context.Background(), validImage(""))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		if _, err := svc.Select(context.Background(), "person", "p1", domain.ImageTypePoster, first.ID); err != nil {
			t.Fatalf("Select(first) returned error: %v", err)
		}
		if _, err := svc.Select(context.Background(), "person", "p1", domain.ImageTypePoster, second.ID); err != nil {
			t.Fatalf("Select(second) returned error: %v", err)
		}

		sel, err := selections.Get(context.Background(), "person", "p1", domain.ImageTypePoster)
		if err != nil {
			t.Fatalf("selections.Get returned error: %v", err)
		}
		if sel.ImageID != second.ID {
			t.Fatalf("selection ImageID = %q, want the second Select's %q", sel.ImageID, second.ID)
		}
	})

	t.Run("selecting a missing image returns ErrNotFound", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		if _, err := svc.Select(context.Background(), "person", "p1", domain.ImageTypePoster, "missing"); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Select on a missing image returned %v, want ErrNotFound", err)
		}
	})

	t.Run("selecting an image that belongs to a different owner is rejected", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		img, err := svc.Create(context.Background(), validImage("i1")) // owner p1
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if _, err := svc.Select(context.Background(), "person", "p2", domain.ImageTypePoster, img.ID); !errors.Is(err, service.ErrImageOwnerMismatch) {
			t.Fatalf("Select across owners returned %v, want ErrImageOwnerMismatch", err)
		}
	})

	t.Run("selecting an image that belongs to a different image type is rejected", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		img, err := svc.Create(context.Background(), validImage("i1")) // ImageTypePoster
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if _, err := svc.Select(context.Background(), "person", "p1", domain.ImageTypeHero, img.ID); !errors.Is(err, service.ErrImageOwnerMismatch) {
			t.Fatalf("Select across image types returned %v, want ErrImageOwnerMismatch", err)
		}
	})
}

func TestImageService_GetSelected(t *testing.T) {
	t.Run("returns the explicitly selected image over any other attached to the slot", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		older, err := svc.Create(context.Background(), validImage(""))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if _, err := svc.Create(context.Background(), validImage("")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		// Deliberately select the older one — proves GetSelected follows the
		// explicit selection, not just "newest wins."
		if _, err := svc.Select(context.Background(), "person", "p1", domain.ImageTypePoster, older.ID); err != nil {
			t.Fatalf("Select returned error: %v", err)
		}

		got, err := svc.GetSelected(context.Background(), "person", "p1", domain.ImageTypePoster)
		if err != nil {
			t.Fatalf("GetSelected returned error: %v", err)
		}
		if got.ID != older.ID {
			t.Fatalf("GetSelected returned %q, want the explicitly selected %q", got.ID, older.ID)
		}
	})

	t.Run("falls back to the newest attached image when nothing has been explicitly selected", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		_, err := svc.Create(context.Background(), validImage(""))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		newest, err := svc.Create(context.Background(), validImage(""))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		got, err := svc.GetSelected(context.Background(), "person", "p1", domain.ImageTypePoster)
		if err != nil {
			t.Fatalf("GetSelected returned error: %v", err)
		}
		if got.ID != newest.ID {
			t.Fatalf("GetSelected (no selection recorded) returned %q, want the newest %q", got.ID, newest.ID)
		}
	})

	t.Run("returns ErrNotFound when the slot has no images at all", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		if _, err := svc.GetSelected(context.Background(), "person", "nobody", domain.ImageTypePoster); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("GetSelected on an empty slot returned %v, want ErrNotFound", err)
		}
	})
}

func TestImageService_List(t *testing.T) {
	t.Run("unfiltered list passes straight through to the repository", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		for _, id := range []string{"i1", "i2"} {
			if _, err := svc.Create(context.Background(), validImage(id)); err != nil {
				t.Fatalf("Create(%q) returned error: %v", id, err)
			}
		}

		images, _, err := svc.List(context.Background(), "", "", "", 10, "")
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(images) != 2 {
			t.Fatalf("List returned %d images, want 2", len(images))
		}
	})

	// Newest-first ordering is now purely a display convenience for the
	// gallery — GetSelected is what's authoritative for "which one is
	// current" — but it's still the order List itself returns.
	t.Run("an owner-scoped list returns the most recently attached image first", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		var ids []string
		for i := 0; i < 3; i++ {
			created, err := svc.Create(context.Background(), validImage(""))
			if err != nil {
				t.Fatalf("Create returned error: %v", err)
			}
			ids = append(ids, created.ID)
		}
		mostRecent := ids[len(ids)-1]

		images, next, err := svc.List(context.Background(), "person", "p1", "", 1, "")
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(images) != 1 || images[0].ID != mostRecent {
			t.Fatalf("List returned %v, want a single-element page led by %q", ids, mostRecent)
		}
		if next == "" {
			t.Fatalf("List returned an empty next_page_token, want one — two more images remain")
		}
	})

	t.Run("an owner-scoped list ignores images belonging to a different owner", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		if _, err := svc.Create(context.Background(), validImage("")); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		other := validImage("")
		other.OwnerID = "p2"
		if _, err := svc.Create(context.Background(), other); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		images, _, err := svc.List(context.Background(), "person", "p2", "", 10, "")
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(images) != 1 || images[0].OwnerID != "p2" {
			t.Fatalf("List returned %v, want exactly the one p2 image", images)
		}
	})

	t.Run("an owner-scoped list filters by image type when given", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		poster, err := svc.Create(context.Background(), validImage(""))
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		hero := validImage("")
		hero.ImageType = domain.ImageTypeHero
		if _, err := svc.Create(context.Background(), hero); err != nil {
			t.Fatalf("Create returned error: %v", err)
		}

		images, _, err := svc.List(context.Background(), "person", "p1", domain.ImageTypePoster, 10, "")
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(images) != 1 || images[0].ID != poster.ID {
			t.Fatalf("List(image_type=poster) returned %v, want exactly the poster %q", images, poster.ID)
		}
	})

	t.Run("pagination continues correctly across the newest-first order", func(t *testing.T) {
		svc, _, _, _ := newTestImageService()

		var ids []string
		for i := 0; i < 3; i++ {
			created, err := svc.Create(context.Background(), validImage(""))
			if err != nil {
				t.Fatalf("Create returned error: %v", err)
			}
			ids = append(ids, created.ID)
		}

		first, next, err := svc.List(context.Background(), "person", "p1", "", 1, "")
		if err != nil {
			t.Fatalf("List (page 1) returned error: %v", err)
		}
		second, next2, err := svc.List(context.Background(), "person", "p1", "", 1, next)
		if err != nil {
			t.Fatalf("List (page 2) returned error: %v", err)
		}
		third, next3, err := svc.List(context.Background(), "person", "p1", "", 1, next2)
		if err != nil {
			t.Fatalf("List (page 3) returned error: %v", err)
		}

		got := []string{first[0].ID, second[0].ID, third[0].ID}
		want := []string{ids[2], ids[1], ids[0]}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("pages returned %v, want %v (newest first)", got, want)
			}
		}
		if next3 != "" {
			t.Fatalf("List (page 3) returned next_page_token %q, want empty — no images remain", next3)
		}
	})

	// listAllForOwner drains the repository in ImageListFetchBatchSize
	// chunks before sorting; shrinking it here is the only way to exercise
	// that loop's "more pages exist" branch without creating hundreds of
	// fixture images.
	t.Run("assembling the owner's set spans more than one repository round trip", func(t *testing.T) {
		original := service.ImageListFetchBatchSize
		service.ImageListFetchBatchSize = 1
		defer func() { service.ImageListFetchBatchSize = original }()

		svc, _, _, _ := newTestImageService()

		for i := 0; i < 3; i++ {
			if _, err := svc.Create(context.Background(), validImage("")); err != nil {
				t.Fatalf("Create returned error: %v", err)
			}
		}

		images, _, err := svc.List(context.Background(), "person", "p1", "", 10, "")
		if err != nil {
			t.Fatalf("List returned error: %v", err)
		}
		if len(images) != 3 {
			t.Fatalf("List returned %d images, want all 3 despite the 1-row-at-a-time fetch", len(images))
		}
	})
}
