package apiconnect_test

import (
	"context"
	"errors"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"

	"connectrpc.com/connect"

	v1 "purser/gen/go/purser/domain/v1"
	apiconnect "purser/internal/api/connect"
)

type fakeImageBlobService struct {
	cacheResult  *service.BlobResult
	cacheErr     error
	uploadResult *service.BlobResult
	uploadErr    error

	gotURL  string
	gotData []byte
}

func (f *fakeImageBlobService) CacheRemoteImage(_ context.Context, url string) (*service.BlobResult, error) {
	f.gotURL = url
	if f.cacheErr != nil {
		return nil, f.cacheErr
	}
	return f.cacheResult, nil
}

func (f *fakeImageBlobService) UploadImage(_ context.Context, data []byte) (*service.BlobResult, error) {
	f.gotData = data
	if f.uploadErr != nil {
		return nil, f.uploadErr
	}
	return f.uploadResult, nil
}

func TestImageBlobHandler_CacheRemoteImage(t *testing.T) {
	t.Run("valid request returns the cached blob", func(t *testing.T) {
		svc := &fakeImageBlobService{cacheResult: &service.BlobResult{
			Key: "unattached/ab/abc.jpg", Width: 100, Height: 50, ContentType: "image/jpeg", SizeBytes: 1024,
		}}
		h := apiconnect.NewImageBlobHandler(svc, nil)

		res, err := h.CacheRemoteImage(context.Background(), connect.NewRequest(&v1.CacheRemoteImageRequest{Url: "https://example.com/cover.jpg"}))
		if err != nil {
			t.Fatalf("CacheRemoteImage returned error: %v", err)
		}
		if svc.gotURL != "https://example.com/cover.jpg" {
			t.Errorf("service called with URL %q, want the request's URL", svc.gotURL)
		}
		if res.Msg.GetBlob().GetKey() != "unattached/ab/abc.jpg" {
			t.Fatalf("CacheRemoteImage returned key %q, want %q", res.Msg.GetBlob().GetKey(), "unattached/ab/abc.jpg")
		}
		if res.Msg.GetBlob().GetWidth() != 100 || res.Msg.GetBlob().GetHeight() != 50 {
			t.Fatalf("CacheRemoteImage returned %dx%d, want 100x50", res.Msg.GetBlob().GetWidth(), res.Msg.GetBlob().GetHeight())
		}
	})

	t.Run("a remote 404 maps to CodeNotFound", func(t *testing.T) {
		svc := &fakeImageBlobService{cacheErr: ports.ErrNotFound}
		h := apiconnect.NewImageBlobHandler(svc, nil)

		_, err := h.CacheRemoteImage(context.Background(), connect.NewRequest(&v1.CacheRemoteImageRequest{Url: "https://example.com/missing.jpg"}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("CacheRemoteImage on a 404 returned code %v, want %v", connect.CodeOf(err), connect.CodeNotFound)
		}
	})
}

func TestImageBlobHandler_UploadImage(t *testing.T) {
	t.Run("valid request returns the uploaded blob", func(t *testing.T) {
		svc := &fakeImageBlobService{uploadResult: &service.BlobResult{
			Key: "unattached/cd/cde.png", Width: 10, Height: 10, ContentType: "image/png", SizeBytes: 512,
		}}
		h := apiconnect.NewImageBlobHandler(svc, nil)

		data := []byte("fake-png-bytes")
		res, err := h.UploadImage(context.Background(), connect.NewRequest(&v1.UploadImageRequest{Data: data}))
		if err != nil {
			t.Fatalf("UploadImage returned error: %v", err)
		}
		if string(svc.gotData) != string(data) {
			t.Errorf("service called with %q, want %q", svc.gotData, data)
		}
		if res.Msg.GetBlob().GetKey() != "unattached/cd/cde.png" {
			t.Fatalf("UploadImage returned key %q, want %q", res.Msg.GetBlob().GetKey(), "unattached/cd/cde.png")
		}
	})

	t.Run("an oversized upload maps through mapError", func(t *testing.T) {
		svc := &fakeImageBlobService{uploadErr: errors.New("adapters/imagestore/local: exceeds max size of 33554432 bytes")}
		h := apiconnect.NewImageBlobHandler(svc, nil)

		_, err := h.UploadImage(context.Background(), connect.NewRequest(&v1.UploadImageRequest{Data: []byte("too big")}))
		if connect.CodeOf(err) != connect.CodeInternal {
			t.Fatalf("UploadImage with an oversized rejection returned code %v, want %v", connect.CodeOf(err), connect.CodeInternal)
		}
	})
}
