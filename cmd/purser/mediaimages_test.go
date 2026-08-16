package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"purser/internal/domain"
	"purser/internal/ports"
	"strings"
	"testing"
)

// fakeImageRepo and fakeImageStore are hand-rolled fakes against the
// narrow imageMetadataGetter/imageBytesGetter interfaces — per
// docs/adr/0003-go-testing-standards.md, mocks are only for ports, never a
// concrete struct, and a CMD-layer handler test fakes its dependencies
// rather than standing up real adapters (the real-adapter case is covered
// by TestNewImageHandler_Integration below, per issue #651's AC).

type fakeImageRepo struct {
	img *domain.Image
	err error
}

func (f *fakeImageRepo) Get(_ context.Context, _ string) (*domain.Image, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.img, nil
}

type fakeImageStore struct {
	data []byte
	err  error
}

func (f *fakeImageStore) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(strings.NewReader(string(f.data))), nil
}

// pngBytes is a minimal valid PNG header — enough for http.DetectContentType
// to sniff "image/png".
var pngBytes = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}

func newTestMux(repo imageMetadataGetter, store imageBytesGetter) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /media/images/{id}", newImageHandler(repo, store, nil))
	return mux
}

func TestNewImageHandler_ServesBytesWithHeaders(t *testing.T) {
	id := "0198c1b0-0000-7000-8000-000000000000"
	repo := &fakeImageRepo{img: &domain.Image{ID: id, URL: "key"}}
	store := &fakeImageStore{data: pngBytes}
	mux := newTestMux(repo, store)

	req := httptest.NewRequest(http.MethodGet, "/media/images/"+id, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Body.Bytes(), pngBytes; string(got) != string(want) {
		t.Fatalf("body = %v, want %v", got, want)
	}
	if got, want := rec.Header().Get("Content-Type"), "image/png"; got != want {
		t.Fatalf("Content-Type = %q, want %q", got, want)
	}
	if got := rec.Header().Get("Cache-Control"); got == "" {
		t.Fatal("Cache-Control header is empty, want a long-lived cache directive")
	}
}

func TestNewImageHandler_MissingImageRowReturns404(t *testing.T) {
	id := "0198c1b0-0000-7000-8000-000000000001"
	repo := &fakeImageRepo{err: ports.ErrNotFound}
	store := &fakeImageStore{}
	mux := newTestMux(repo, store)

	req := httptest.NewRequest(http.MethodGet, "/media/images/"+id, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestNewImageHandler_MissingBlobKeyReturns404(t *testing.T) {
	id := "0198c1b0-0000-7000-8000-000000000002"
	repo := &fakeImageRepo{img: &domain.Image{ID: id, URL: "key"}}
	store := &fakeImageStore{err: ports.ErrNotFound}
	mux := newTestMux(repo, store)

	req := httptest.NewRequest(http.MethodGet, "/media/images/"+id, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestNewImageHandler_MalformedIDReturns404BeforeRepoLookup(t *testing.T) {
	repo := &fakeImageRepo{err: errors.New("imageRepo.Get must never be called for a malformed id")}
	store := &fakeImageStore{}
	mux := newTestMux(repo, store)

	// Percent-encoded so http.ServeMux's own path-cleaning redirect (which
	// would otherwise 307 a literal "../.." segment before it ever reaches
	// a handler) doesn't mask what this test is actually proving: that the
	// handler's own uuid.Parse check rejects a traversal-shaped id that did
	// reach it, before it's passed to imageRepo.Get.
	req := httptest.NewRequest(http.MethodGet, "/media/images/..%2F..%2Fetc%2Fpasswd", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestNewImageHandler_RepoErrorReturns500(t *testing.T) {
	id := "0198c1b0-0000-7000-8000-000000000003"
	repo := &fakeImageRepo{err: errors.New("boom")}
	store := &fakeImageStore{}
	mux := newTestMux(repo, store)

	req := httptest.NewRequest(http.MethodGet, "/media/images/"+id, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestNewImageHandler_StoreErrorReturns500(t *testing.T) {
	id := "0198c1b0-0000-7000-8000-000000000004"
	repo := &fakeImageRepo{img: &domain.Image{ID: id, URL: "key"}}
	store := &fakeImageStore{err: errors.New("boom")}
	mux := newTestMux(repo, store)

	req := httptest.NewRequest(http.MethodGet, "/media/images/"+id, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
