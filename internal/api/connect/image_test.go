package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	v1 "purser/gen/go/purser/domain/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeImageService struct {
	byID      map[string]*domain.Image
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	listErr   error
}

func newFakeImageService() *fakeImageService {
	return &fakeImageService{byID: make(map[string]*domain.Image)}
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

func (f *fakeImageService) List(_ context.Context, _, _ string, _ int, _ string) ([]*domain.Image, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	images := make([]*domain.Image, 0, len(f.byID))
	for _, img := range f.byID {
		images = append(images, img)
	}
	return images, "", nil
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
