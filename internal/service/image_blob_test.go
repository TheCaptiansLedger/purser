package service_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"purser/internal/ports"
	"purser/internal/service"
	"testing"
)

// fakeImageFetcher is a minimal ports.ImageFetcher double, per
// docs/adr/0003-go-testing-standards.md's "services fake the ports they
// consume" rule.
type fakeImageFetcher struct {
	body    []byte
	err     error
	gotURL  string
	fetches int
}

var _ ports.ImageFetcher = (*fakeImageFetcher)(nil)

func (f *fakeImageFetcher) Fetch(_ context.Context, url string) (io.ReadCloser, error) {
	f.gotURL = url
	f.fetches++
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(bytes.NewReader(f.body)), nil
}

// fakeImageStore is a minimal ports.ImageStore double, shared with
// image_test.go — ImageService.Delete is the other caller, exercising
// Delete/deleteErr/deleted below.
type fakeImageStore struct {
	key string
	err error

	gotOwnerType string
	gotID        string
	gotBytes     []byte

	// deleteErr, when set, is returned by the next Delete call — used by
	// image_test.go to prove ImageService.Delete's row delete still
	// succeeds even when the blob cleanup fails (the accepted, bounded
	// orphan-file risk).
	deleteErr error
	deleted   []string
}

var _ ports.ImageStore = (*fakeImageStore)(nil)

func (f *fakeImageStore) Put(_ context.Context, ownerType, id string, r io.Reader) (string, error) {
	f.gotOwnerType = ownerType
	f.gotID = id
	if f.err != nil {
		return "", f.err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	f.gotBytes = data
	return f.key, nil
}

func (f *fakeImageStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeImageStore) Delete(_ context.Context, key string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, key)
	return nil
}

// pngFixture returns a small, valid PNG's encoded bytes with the given
// dimensions, for exercising width/height sniffing.
func pngFixture(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding PNG fixture: %v", err)
	}
	return buf.Bytes()
}

func TestImageBlobService_CacheRemoteImage_Success(t *testing.T) {
	data := pngFixture(t, 4, 3)
	fetcher := &fakeImageFetcher{body: data}
	store := &fakeImageStore{key: "unattached/ab/abc123.png"}
	s := service.NewImageBlobService(fetcher, store)

	got, err := s.CacheRemoteImage(context.Background(), "https://example.com/cover.png")
	if err != nil {
		t.Fatalf("CacheRemoteImage returned error: %v", err)
	}
	if fetcher.gotURL != "https://example.com/cover.png" {
		t.Errorf("Fetch called with %q, want the passed URL", fetcher.gotURL)
	}
	if got.Key != store.key {
		t.Errorf("Key = %q, want %q", got.Key, store.key)
	}
	if got.Width != 4 || got.Height != 3 {
		t.Errorf("Width/Height = %d/%d, want 4/3", got.Width, got.Height)
	}
	if got.ContentType != "image/png" {
		t.Errorf("ContentType = %q, want image/png", got.ContentType)
	}
	if got.SizeBytes != int64(len(data)) {
		t.Errorf("SizeBytes = %d, want %d", got.SizeBytes, len(data))
	}
	if !bytes.Equal(store.gotBytes, data) {
		t.Error("Put was not given the same bytes Fetch returned")
	}
	if store.gotOwnerType == "" || store.gotID == "" {
		t.Error("Put was called with an empty ownerType or id")
	}
}

func TestImageBlobService_CacheRemoteImage_PropagatesNotFound(t *testing.T) {
	fetcher := &fakeImageFetcher{err: fmt.Errorf("fetching: %w", ports.ErrNotFound)}
	store := &fakeImageStore{}
	s := service.NewImageBlobService(fetcher, store)

	_, err := s.CacheRemoteImage(context.Background(), "https://example.com/missing.png")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("CacheRemoteImage returned %v, want ports.ErrNotFound", err)
	}
}

func TestImageBlobService_UploadImage_Success(t *testing.T) {
	data := pngFixture(t, 8, 6)
	store := &fakeImageStore{key: "unattached/cd/cde456.png"}
	s := service.NewImageBlobService(&fakeImageFetcher{}, store)

	got, err := s.UploadImage(context.Background(), data)
	if err != nil {
		t.Fatalf("UploadImage returned error: %v", err)
	}
	if got.Key != store.key {
		t.Errorf("Key = %q, want %q", got.Key, store.key)
	}
	if got.Width != 8 || got.Height != 6 {
		t.Errorf("Width/Height = %d/%d, want 8/6", got.Width, got.Height)
	}
	if got.ContentType != "image/png" {
		t.Errorf("ContentType = %q, want image/png", got.ContentType)
	}
}

func TestImageBlobService_UploadImage_PropagatesOversizedRejection(t *testing.T) {
	wantErr := errors.New("adapters/imagestore/local: exceeds max size")
	store := &fakeImageStore{err: wantErr}
	s := service.NewImageBlobService(&fakeImageFetcher{}, store)

	_, err := s.UploadImage(context.Background(), pngFixture(t, 1, 1))
	if !errors.Is(err, wantErr) {
		t.Fatalf("UploadImage returned %v, want %v", err, wantErr)
	}
}
