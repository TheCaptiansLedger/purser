package service

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"sort"
)

// ImageListFetchBatchSize is how many rows ImageService.List asks the
// repository for per round trip while assembling the full owner-scoped
// set to sort — see the ownerID branch of List. Owners realistically carry
// a handful of images, so a couple of round trips at worst. Exported
// (unlike a tunable would be) purely as a test seam: image_test.go
// shrinks it to force the multi-batch loop in listAllForOwner without
// creating hundreds of fixture images.
var ImageListFetchBatchSize = 200

// ErrImageOwnerMismatch is returned by Select when imageID names a real
// Image that belongs to a different owner or image type than the slot
// being selected for — mapped to connect.CodeInvalidArgument by
// internal/api/connect's shared error-mapping, same convention
// service.ErrUnsupportedProtocol already established.
var ErrImageOwnerMismatch = errors.New("service: image does not belong to the given owner and image type")

// ImageService orchestrates domain.Image against a ports.ImageRepository,
// plus the two things layered on top of plain Image CRUD: which Image is
// the current one for a given owner+slot (ports.ImageSelectionRepository),
// and actually freeing the underlying bytes on delete (ports.ImageStore).
// See PersonService for the base conventions this follows.
type ImageService struct {
	repo       ports.ImageRepository
	selections ports.ImageSelectionRepository
	store      ports.ImageStore
}

// NewImageService constructs an ImageService backed by repo, selections,
// and store.
func NewImageService(repo ports.ImageRepository, selections ports.ImageSelectionRepository, store ports.ImageStore) *ImageService {
	return &ImageService{repo: repo, selections: selections, store: store}
}

// Create assigns img a server-generated ID (see
// docs/adr/0020-server-generated-kernel-entity-ids.md), validates it, and
// persists it. Does not select it — attaching and selecting are two
// separate calls the caller composes, same "composition lives in the
// client" rule docs/technical/image-caching-and-serving.md already
// applies to the blob-then-metadata sequence.
func (s *ImageService) Create(ctx context.Context, img *domain.Image) (*domain.Image, error) {
	img.ID = domain.NewID()
	if err := img.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, img); err != nil {
		return nil, err
	}
	return img, nil
}

// Get returns the Image with the given ID, or ports.ErrNotFound.
func (s *ImageService) Get(ctx context.Context, id string) (*domain.Image, error) {
	return s.repo.Get(ctx, id)
}

// Update validates img and persists it in place of the existing record.
func (s *ImageService) Update(ctx context.Context, img *domain.Image) (*domain.Image, error) {
	if err := img.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, img); err != nil {
		return nil, err
	}
	return img, nil
}

// Delete removes the Image row, frees its underlying ImageStore bytes
// (best-effort — a failure here leaves an orphaned file, the same
// accepted, bounded risk the create path already carries, and does not
// roll back the row delete that already succeeded), and — if the deleted
// image was the one selected for its owner+slot — reassigns that
// selection to whatever's now the newest remaining image, or clears it if
// nothing's left. Returns ports.ErrNotFound if id doesn't exist.
func (s *ImageService) Delete(ctx context.Context, id string) error {
	img, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	_ = s.store.Delete(ctx, img.URL)

	return s.reassignSelectionAfterDelete(ctx, img)
}

func (s *ImageService) reassignSelectionAfterDelete(ctx context.Context, deleted *domain.Image) error {
	sel, err := s.selections.Get(ctx, deleted.OwnerType, deleted.OwnerID, deleted.ImageType)
	if errors.Is(err, ports.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if sel.ImageID != deleted.ID {
		return nil
	}

	remaining, _, err := s.List(ctx, deleted.OwnerType, deleted.OwnerID, deleted.ImageType, 1, "")
	if err != nil {
		return err
	}
	if len(remaining) == 0 {
		return s.selections.Delete(ctx, deleted.OwnerType, deleted.OwnerID, deleted.ImageType)
	}
	sel.ImageID = remaining[0].ID
	return s.selections.Update(ctx, sel)
}

// Select records imageID as the current image for the
// (ownerType, ownerID, imageType) slot — the action behind both "attach a
// new photo" (the client calls this right after Create) and "use this
// one" in a gallery of already-attached images. imageID must already be
// an Image belonging to exactly that owner and image type, or
// ErrImageOwnerMismatch.
func (s *ImageService) Select(ctx context.Context, ownerType, ownerID string, imageType domain.ImageType, imageID string) (*domain.ImageSelection, error) {
	img, err := s.repo.Get(ctx, imageID)
	if err != nil {
		return nil, err
	}
	if img.OwnerType != ownerType || img.OwnerID != ownerID || img.ImageType != imageType {
		return nil, ErrImageOwnerMismatch
	}

	sel := &domain.ImageSelection{OwnerType: ownerType, OwnerID: ownerID, ImageType: imageType, ImageID: imageID}
	if err := sel.Validate(); err != nil {
		return nil, err
	}
	if err := s.setSelection(ctx, sel); err != nil {
		return nil, err
	}
	return sel, nil
}

// setSelection creates the slot's selection if none exists yet, otherwise
// overwrites which Image it points at. ImageSelectionRepository only
// exposes plain Create/Get/Update (see its own doc comment, same ISP
// reasoning as every other narrow port here) — this is the one place
// that turns those into "set, whichever it takes."
func (s *ImageService) setSelection(ctx context.Context, sel *domain.ImageSelection) error {
	_, err := s.selections.Get(ctx, sel.OwnerType, sel.OwnerID, sel.ImageType)
	switch {
	case err == nil:
		return s.selections.Update(ctx, sel)
	case errors.Is(err, ports.ErrNotFound):
		if createErr := s.selections.Create(ctx, sel); createErr != nil {
			if errors.Is(createErr, ports.ErrConflict) {
				// Lost a race against a concurrent Select for the same
				// slot — fall back to Update; last writer still wins.
				return s.selections.Update(ctx, sel)
			}
			return createErr
		}
		return nil
	default:
		return err
	}
}

// GetSelected returns the current Image for an owner+slot — the one call
// every screen that renders "this owner's photo/poster/whatever" should
// use, instead of guessing from List's row order. Falls back to the
// newest-attached image for the slot when nothing has been explicitly
// selected yet (every image attached before Select existed), so no
// migration is needed and no owner silently loses their picture. Returns
// ports.ErrNotFound if the slot has no images at all.
func (s *ImageService) GetSelected(ctx context.Context, ownerType, ownerID string, imageType domain.ImageType) (*domain.Image, error) {
	sel, err := s.selections.Get(ctx, ownerType, ownerID, imageType)
	switch {
	case err == nil:
		img, getErr := s.repo.Get(ctx, sel.ImageID)
		if getErr == nil {
			return img, nil
		}
		if !errors.Is(getErr, ports.ErrNotFound) {
			return nil, getErr
		}
		// The selection points at a row that's gone — Delete keeps these in
		// sync, so this shouldn't happen, but fall back below rather than
		// surface a broken pointer.
	case errors.Is(err, ports.ErrNotFound):
		// No selection recorded yet — fall back below.
	default:
		return nil, err
	}

	images, _, err := s.List(ctx, ownerType, ownerID, imageType, 1, "")
	if err != nil {
		return nil, err
	}
	if len(images) == 0 {
		return nil, ports.ErrNotFound
	}
	return images[0], nil
}

// List returns a page of Image records, optionally filtered by ownerType,
// ownerID, and/or imageType (imageType only takes effect when ownerID is
// also given — see below).
//
// When ownerID is given, the owner's full set is fetched and sorted
// ID-descending (IDs are time-ordered, docs/adr/0020, so this is "most
// recently attached first") before imageType filtering and the requested
// page are applied — this is what feeds a gallery of everything ever
// attached to one owner+slot. The repository/store stay unordered per
// docs/adr/0012 — ordering is this service's business rule to apply, not
// theirs.
func (s *ImageService) List(ctx context.Context, ownerType, ownerID string, imageType domain.ImageType, pageSize int, pageToken string) ([]*domain.Image, string, error) {
	if ownerID == "" {
		return s.repo.List(ctx, ownerType, ownerID, pageSize, pageToken)
	}

	all, err := s.listAllForOwner(ctx, ownerType, ownerID)
	if err != nil {
		return nil, "", err
	}
	if imageType != "" {
		all = filterByImageType(all, imageType)
	}

	sort.SliceStable(all, func(i, j int) bool { return all[i].ID > all[j].ID })

	return paginateImages(all, pageSize, pageToken)
}

func filterByImageType(images []*domain.Image, imageType domain.ImageType) []*domain.Image {
	out := make([]*domain.Image, 0, len(images))
	for _, img := range images {
		if img.ImageType == imageType {
			out = append(out, img)
		}
	}
	return out
}

// listAllForOwner drains every page the repository has for
// (ownerType, ownerID) — owners realistically carry a handful of images,
// so this is a couple of round trips at worst, not a tunable.
func (s *ImageService) listAllForOwner(ctx context.Context, ownerType, ownerID string) ([]*domain.Image, error) {
	var all []*domain.Image
	token := ""
	for {
		batch, next, err := s.repo.List(ctx, ownerType, ownerID, ImageListFetchBatchSize, token)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if next == "" {
			return all, nil
		}
		token = next
	}
}

// paginateImages windows a slice already in the order callers should see
// into one page, using the same "token = last returned ID" cursor
// contract store.FilteredRepository.List uses — the slice just isn't in
// ID order here, so the store's own ID-range pagination can't be reused
// directly.
func paginateImages(images []*domain.Image, pageSize int, pageToken string) ([]*domain.Image, string, error) {
	if pageSize <= 0 {
		pageSize = 50
	}

	start := 0
	if pageToken != "" {
		for i, img := range images {
			if img.ID == pageToken {
				start = i + 1
				break
			}
		}
	}

	end := start + pageSize
	if end > len(images) {
		end = len(images)
	}
	page := images[start:end]

	next := ""
	if end < len(images) {
		next = page[len(page)-1].ID
	}
	return page, next, nil
}
