package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeImageService struct {
	byID       map[string]*domain.Image
	selections map[string]*domain.ImageSelection
	createErr  error
	getErr     error
	updateErr  error
	deleteErr  error
	listErr    error
	selectErr  error
	getSelErr  error
}

func newFakeImageService() *fakeImageService {
	return &fakeImageService{byID: make(map[string]*domain.Image), selections: make(map[string]*domain.ImageSelection)}
}

func imageSlotKey(ownerType, ownerID string, imageType domain.ImageType) string {
	return ownerType + "/" + ownerID + "/" + string(imageType)
}

func (f *fakeImageService) Create(_ context.Context, img *domain.Image) (*domain.Image, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.byID[img.ID] = img
	return img, nil
}

func (f *fakeImageService) Get(_ context.Context, id string) (*domain.Image, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	img, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return img, nil
}

func (f *fakeImageService) Update(_ context.Context, img *domain.Image) (*domain.Image, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byID[img.ID] = img
	return img, nil
}

func (f *fakeImageService) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeImageService) List(_ context.Context, _, _ string, _ domain.ImageType, _ int, _ string) ([]*domain.Image, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	images := make([]*domain.Image, 0, len(f.byID))
	for _, img := range f.byID {
		images = append(images, img)
	}
	return images, "", nil
}

func (f *fakeImageService) Select(_ context.Context, ownerType, ownerID string, imageType domain.ImageType, imageID string) (*domain.ImageSelection, error) {
	if f.selectErr != nil {
		return nil, f.selectErr
	}
	sel := &domain.ImageSelection{OwnerType: ownerType, OwnerID: ownerID, ImageType: imageType, ImageID: imageID}
	f.selections[imageSlotKey(ownerType, ownerID, imageType)] = sel
	return sel, nil
}

func (f *fakeImageService) GetSelected(_ context.Context, ownerType, ownerID string, imageType domain.ImageType) (*domain.Image, error) {
	if f.getSelErr != nil {
		return nil, f.getSelErr
	}
	sel, ok := f.selections[imageSlotKey(ownerType, ownerID, imageType)]
	if !ok {
		return nil, ports.ErrNotFound
	}
	img, ok := f.byID[sel.ImageID]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return img, nil
}

func validProtoImage(id string) *v1.Image {
	return &v1.Image{Id: id, OwnerType: "person", OwnerId: "p1", ImageType: "poster", Url: "https://example.com/i.jpg"}
}

func TestImageHandler_CreateImage(t *testing.T) {
	t.Run("valid request returns the created image", func(t *testing.T) {
		svc := newFakeImageService()
		h := apiconnect.NewImageHandler(svc, nil)

		res, err := h.CreateImage(context.Background(), connect.NewRequest(&v1.CreateImageRequest{Image: validProtoImage("i1")}))
		if err != nil {
			t.Fatalf("CreateImage returned error: %v", err)
		}
		if res.Msg.GetImage().GetId() != "i1" {
			t.Fatalf("CreateImage returned ID %q, want %q", res.Msg.GetImage().GetId(), "i1")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakeImageService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "URL", Rule: "required", Value: ""}}}
		h := apiconnect.NewImageHandler(svc, nil)

		_, err := h.CreateImage(context.Background(), connect.NewRequest(&v1.CreateImageRequest{Image: validProtoImage("i1")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateImage with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestImageHandler_GetImage(t *testing.T) {
	svc := newFakeImageService()
	h := apiconnect.NewImageHandler(svc, nil)
	svc.byID["i1"] = &domain.Image{ID: "i1", URL: "https://example.com/existing.jpg"}

	res, err := h.GetImage(context.Background(), connect.NewRequest(&v1.GetImageRequest{Id: "i1"}))
	if err != nil {
		t.Fatalf("GetImage returned error: %v", err)
	}
	if res.Msg.GetImage().GetUrl() != "https://example.com/existing.jpg" {
		t.Fatalf("GetImage returned URL %q, want %q", res.Msg.GetImage().GetUrl(), "https://example.com/existing.jpg")
	}

	_, err = h.GetImage(context.Background(), connect.NewRequest(&v1.GetImageRequest{Id: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetImage on missing ID returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestImageHandler_UpdateImage(t *testing.T) {
	t.Run("field mask restricts the applied fields", func(t *testing.T) {
		svc := newFakeImageService()
		h := apiconnect.NewImageHandler(svc, nil)
		svc.byID["i1"] = &domain.Image{ID: "i1", URL: "https://example.com/original.jpg", Priority: 1}

		req := &v1.UpdateImageRequest{
			Image:      &v1.Image{Id: "i1", Url: "https://example.com/new.jpg", Priority: 99},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"url"}},
		}
		res, err := h.UpdateImage(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateImage returned error: %v", err)
		}
		if res.Msg.GetImage().GetUrl() != "https://example.com/new.jpg" {
			t.Fatalf("UpdateImage applied Url %q, want %q", res.Msg.GetImage().GetUrl(), "https://example.com/new.jpg")
		}
		if res.Msg.GetImage().GetPriority() != 1 {
			t.Fatalf("UpdateImage touched Priority: got %d", res.Msg.GetImage().GetPriority())
		}
	})

	t.Run("get failure maps through mapError", func(t *testing.T) {
		svc := newFakeImageService()
		h := apiconnect.NewImageHandler(svc, nil)

		_, err := h.UpdateImage(context.Background(), connect.NewRequest(&v1.UpdateImageRequest{Image: &v1.Image{Id: "missing"}}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("UpdateImage on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestImageHandler_DeleteImage(t *testing.T) {
	t.Run("valid delete succeeds", func(t *testing.T) {
		svc := newFakeImageService()
		h := apiconnect.NewImageHandler(svc, nil)
		svc.byID["i1"] = &domain.Image{ID: "i1"}

		if _, err := h.DeleteImage(context.Background(), connect.NewRequest(&v1.DeleteImageRequest{Id: "i1"})); err != nil {
			t.Fatalf("DeleteImage returned error: %v", err)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeImageService()
		svc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewImageHandler(svc, nil)

		_, err := h.DeleteImage(context.Background(), connect.NewRequest(&v1.DeleteImageRequest{Id: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeleteImage on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestImageHandler_ListImages(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeImageService()
		h := apiconnect.NewImageHandler(svc, nil)
		svc.byID["i1"] = &domain.Image{ID: "i1"}
		svc.byID["i2"] = &domain.Image{ID: "i2"}

		res, err := h.ListImages(context.Background(), connect.NewRequest(&v1.ListImagesRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListImages returned error: %v", err)
		}
		if len(res.Msg.GetImages()) != 2 {
			t.Fatalf("ListImages returned %d images, want 2", len(res.Msg.GetImages()))
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeImageService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewImageHandler(svc, nil)

		_, err := h.ListImages(context.Background(), connect.NewRequest(&v1.ListImagesRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListImages with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}

func TestImageHandler_SelectImage(t *testing.T) {
	t.Run("valid select returns the now-selected image", func(t *testing.T) {
		svc := newFakeImageService()
		h := apiconnect.NewImageHandler(svc, nil)
		svc.byID["i1"] = &domain.Image{ID: "i1", OwnerType: "person", OwnerID: "p1", ImageType: "photo", URL: "https://example.com/i1.jpg"}

		req := &v1.SelectImageRequest{OwnerType: "person", OwnerId: "p1", ImageType: "photo", ImageId: "i1"}
		res, err := h.SelectImage(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("SelectImage returned error: %v", err)
		}
		if res.Msg.GetImage().GetId() != "i1" {
			t.Fatalf("SelectImage returned image %q, want %q", res.Msg.GetImage().GetId(), "i1")
		}
	})

	t.Run("an owner-mismatch error from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakeImageService()
		svc.selectErr = service.ErrImageOwnerMismatch
		h := apiconnect.NewImageHandler(svc, nil)

		req := &v1.SelectImageRequest{OwnerType: "person", OwnerId: "p1", ImageType: "photo", ImageId: "i1"}
		_, err := h.SelectImage(context.Background(), connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("SelectImage with ErrImageOwnerMismatch returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestImageHandler_GetSelectedImage(t *testing.T) {
	t.Run("returns the selected image for the slot", func(t *testing.T) {
		svc := newFakeImageService()
		h := apiconnect.NewImageHandler(svc, nil)
		svc.byID["i1"] = &domain.Image{ID: "i1", OwnerType: "person", OwnerID: "p1", ImageType: "photo", URL: "https://example.com/i1.jpg"}
		svc.selections[imageSlotKey("person", "p1", "photo")] = &domain.ImageSelection{OwnerType: "person", OwnerID: "p1", ImageType: "photo", ImageID: "i1"}

		req := &v1.GetSelectedImageRequest{OwnerType: "person", OwnerId: "p1", ImageType: "photo"}
		res, err := h.GetSelectedImage(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("GetSelectedImage returned error: %v", err)
		}
		if res.Msg.GetImage().GetId() != "i1" {
			t.Fatalf("GetSelectedImage returned image %q, want %q", res.Msg.GetImage().GetId(), "i1")
		}
	})

	t.Run("no selection maps to CodeNotFound", func(t *testing.T) {
		svc := newFakeImageService()
		h := apiconnect.NewImageHandler(svc, nil)

		req := &v1.GetSelectedImageRequest{OwnerType: "person", OwnerId: "nobody", ImageType: "photo"}
		_, err := h.GetSelectedImage(context.Background(), connect.NewRequest(req))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("GetSelectedImage on an empty slot returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}
