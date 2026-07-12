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

type fakeMediaFileService struct {
	byID      map[string]*domain.MediaFile
	createErr error
	getErr    error
	updateErr error
	deleteErr error
	listErr   error
}

func newFakeMediaFileService() *fakeMediaFileService {
	return &fakeMediaFileService{byID: make(map[string]*domain.MediaFile)}
}

func (f *fakeMediaFileService) Create(_ context.Context, m *domain.MediaFile) (*domain.MediaFile, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.byID[m.ID] = m
	return m, nil
}

func (f *fakeMediaFileService) Get(_ context.Context, id string) (*domain.MediaFile, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	m, ok := f.byID[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	return m, nil
}

func (f *fakeMediaFileService) Update(_ context.Context, m *domain.MediaFile) (*domain.MediaFile, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.byID[m.ID] = m
	return m, nil
}

func (f *fakeMediaFileService) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byID, id)
	return nil
}

func (f *fakeMediaFileService) List(_ context.Context, _ int, _ string) ([]*domain.MediaFile, string, error) {
	if f.listErr != nil {
		return nil, "", f.listErr
	}
	files := make([]*domain.MediaFile, 0, len(f.byID))
	for _, m := range f.byID {
		files = append(files, m)
	}
	return files, "", nil
}

func validProtoMediaFile(id string) *v1.MediaFile {
	return &v1.MediaFile{Id: id, ItemId: "item1", Path: "/media/test.mkv"}
}

func TestMediaFileHandler_CreateMediaFile(t *testing.T) {
	t.Run("valid request returns the created media file", func(t *testing.T) {
		svc := newFakeMediaFileService()
		h := apiconnect.NewMediaFileHandler(svc, nil)

		res, err := h.CreateMediaFile(context.Background(), connect.NewRequest(&v1.CreateMediaFileRequest{MediaFile: validProtoMediaFile("m1")}))
		if err != nil {
			t.Fatalf("CreateMediaFile returned error: %v", err)
		}
		if res.Msg.GetMediaFile().GetId() != "m1" {
			t.Fatalf("CreateMediaFile returned ID %q, want %q", res.Msg.GetMediaFile().GetId(), "m1")
		}
	})

	t.Run("a ValidationError from the service maps to CodeInvalidArgument", func(t *testing.T) {
		svc := newFakeMediaFileService()
		svc.createErr = &domain.ValidationError{Errors: []domain.FieldError{{Field: "Path", Rule: "required", Value: ""}}}
		h := apiconnect.NewMediaFileHandler(svc, nil)

		_, err := h.CreateMediaFile(context.Background(), connect.NewRequest(&v1.CreateMediaFileRequest{MediaFile: validProtoMediaFile("m1")}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("CreateMediaFile with a ValidationError returned code %v, want %v", connect.CodeOf(err), connect.CodeInvalidArgument)
		}
	})
}

func TestMediaFileHandler_GetMediaFile(t *testing.T) {
	svc := newFakeMediaFileService()
	h := apiconnect.NewMediaFileHandler(svc, nil)
	svc.byID["m1"] = &domain.MediaFile{ID: "m1", Path: "/media/existing.mkv"}

	res, err := h.GetMediaFile(context.Background(), connect.NewRequest(&v1.GetMediaFileRequest{Id: "m1"}))
	if err != nil {
		t.Fatalf("GetMediaFile returned error: %v", err)
	}
	if res.Msg.GetMediaFile().GetPath() != "/media/existing.mkv" {
		t.Fatalf("GetMediaFile returned Path %q, want %q", res.Msg.GetMediaFile().GetPath(), "/media/existing.mkv")
	}

	_, err = h.GetMediaFile(context.Background(), connect.NewRequest(&v1.GetMediaFileRequest{Id: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("GetMediaFile on missing ID returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
	}
}

func TestMediaFileHandler_UpdateMediaFile(t *testing.T) {
	t.Run("field mask restricts the applied fields", func(t *testing.T) {
		svc := newFakeMediaFileService()
		h := apiconnect.NewMediaFileHandler(svc, nil)
		svc.byID["m1"] = &domain.MediaFile{ID: "m1", Path: "/media/original.mkv", Quality: "720p"}

		req := &v1.UpdateMediaFileRequest{
			MediaFile:  &v1.MediaFile{Id: "m1", Path: "/media/new.mkv", Quality: "should be ignored"},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"path"}},
		}
		res, err := h.UpdateMediaFile(context.Background(), connect.NewRequest(req))
		if err != nil {
			t.Fatalf("UpdateMediaFile returned error: %v", err)
		}
		if res.Msg.GetMediaFile().GetPath() != "/media/new.mkv" {
			t.Fatalf("UpdateMediaFile applied Path %q, want %q", res.Msg.GetMediaFile().GetPath(), "/media/new.mkv")
		}
		if res.Msg.GetMediaFile().GetQuality() != "720p" {
			t.Fatalf("UpdateMediaFile touched Quality: got %q", res.Msg.GetMediaFile().GetQuality())
		}
	})

	t.Run("get failure maps through mapError", func(t *testing.T) {
		svc := newFakeMediaFileService()
		h := apiconnect.NewMediaFileHandler(svc, nil)

		_, err := h.UpdateMediaFile(context.Background(), connect.NewRequest(&v1.UpdateMediaFileRequest{MediaFile: &v1.MediaFile{Id: "missing"}}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("UpdateMediaFile on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestMediaFileHandler_DeleteMediaFile(t *testing.T) {
	t.Run("valid delete succeeds", func(t *testing.T) {
		svc := newFakeMediaFileService()
		h := apiconnect.NewMediaFileHandler(svc, nil)
		svc.byID["m1"] = &domain.MediaFile{ID: "m1"}

		if _, err := h.DeleteMediaFile(context.Background(), connect.NewRequest(&v1.DeleteMediaFileRequest{Id: "m1"})); err != nil {
			t.Fatalf("DeleteMediaFile returned error: %v", err)
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeMediaFileService()
		svc.deleteErr = ports.ErrNotFound
		h := apiconnect.NewMediaFileHandler(svc, nil)

		_, err := h.DeleteMediaFile(context.Background(), connect.NewRequest(&v1.DeleteMediaFileRequest{Id: "missing"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("DeleteMediaFile on missing id returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestMediaFileHandler_ListMediaFiles(t *testing.T) {
	t.Run("valid list succeeds", func(t *testing.T) {
		svc := newFakeMediaFileService()
		h := apiconnect.NewMediaFileHandler(svc, nil)
		svc.byID["m1"] = &domain.MediaFile{ID: "m1"}
		svc.byID["m2"] = &domain.MediaFile{ID: "m2"}

		res, err := h.ListMediaFiles(context.Background(), connect.NewRequest(&v1.ListMediaFilesRequest{PageSize: 10}))
		if err != nil {
			t.Fatalf("ListMediaFiles returned error: %v", err)
		}
		if len(res.Msg.GetMediaFiles()) != 2 {
			t.Fatalf("ListMediaFiles returned %d media files, want 2", len(res.Msg.GetMediaFiles()))
		}
	})

	t.Run("service error maps through mapError", func(t *testing.T) {
		svc := newFakeMediaFileService()
		svc.listErr = errors.New("boom")
		h := apiconnect.NewMediaFileHandler(svc, nil)

		_, err := h.ListMediaFiles(context.Background(), connect.NewRequest(&v1.ListMediaFilesRequest{PageSize: 10}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("ListMediaFiles with a service error returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
