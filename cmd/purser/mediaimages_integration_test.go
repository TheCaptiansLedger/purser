package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/datastore"
	dsbadger "purser/internal/adapters/datastore/badger"
	imagestorelocal "purser/internal/adapters/imagestore/local"
	storeimage "purser/internal/adapters/store/image"
	"purser/internal/domain"
	"testing"
)

// TestNewImageHandler_Integration exercises the real ImageRepository
// (Badger-backed) and real ImageStore (local-filesystem) adapters, per
// issue #651's acceptance criteria: a real CreateImage + ImageStore.Put,
// then a real HTTP GET against the handler, asserting byte-for-byte match
// and the expected headers.
func TestNewImageHandler_Integration(t *testing.T) {
	ctx := context.Background()

	db, err := dsbadger.Open(dsbadger.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("badger.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	})
	ds, err := dsbadger.New("test", db)
	if err != nil {
		t.Fatalf("badger.New returned error: %v", err)
	}
	var _ datastore.Datastore = ds

	imageRepo, err := storeimage.New("image", ds)
	if err != nil {
		t.Fatalf("storeimage.New returned error: %v", err)
	}

	imageStore, err := imagestorelocal.New("image", t.TempDir())
	if err != nil {
		t.Fatalf("imagestorelocal.New returned error: %v", err)
	}

	// A minimal valid JPEG header, enough for http.DetectContentType to
	// sniff "image/jpeg" and for the round-trip byte comparison below.
	want := append([]byte{0xff, 0xd8, 0xff, 0xe0, 0, 0, 0, 0}, []byte("purser-test-image-bytes")...)

	key, err := imageStore.Put(ctx, "person", "test-owner", bytes.NewReader(want))
	if err != nil {
		t.Fatalf("imageStore.Put returned error: %v", err)
	}

	img := &domain.Image{
		ID:        domain.NewID(),
		OwnerType: "person",
		OwnerID:   "test-owner",
		ImageType: domain.ImageTypePoster,
		URL:       key,
	}
	if err := imageRepo.Create(ctx, img); err != nil {
		t.Fatalf("imageRepo.Create returned error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /media/images/{id}", newImageHandler(imageRepo, imageStore, nil))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/media/images/" + img.ID) //nolint:noctx,gosec // test-only, URL is a fixed httptest.Server address
	if err != nil {
		t.Fatalf("http.Get returned error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("Content-Type = %q, want %q", got, "image/jpeg")
	}
	if got := resp.Header.Get("Cache-Control"); got == "" {
		t.Fatal("Cache-Control header is empty, want a long-lived cache directive")
	}

	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("body = %v, want byte-for-byte match %v", got, want)
	}
}

// TestNewImageHandler_Integration_MissingBlobKeyReturns404 covers the AC's
// second 404 case with a real ImageStore: the Image row exists but the
// underlying blob was never written (or was already deleted).
func TestNewImageHandler_Integration_MissingBlobKeyReturns404(t *testing.T) {
	ctx := context.Background()

	db, err := dsbadger.Open(dsbadger.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("badger.Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close returned error: %v", err)
		}
	})
	ds, err := dsbadger.New("test", db)
	if err != nil {
		t.Fatalf("badger.New returned error: %v", err)
	}

	imageRepo, err := storeimage.New("image", ds)
	if err != nil {
		t.Fatalf("storeimage.New returned error: %v", err)
	}
	imageStore, err := imagestorelocal.New("image", t.TempDir())
	if err != nil {
		t.Fatalf("imagestorelocal.New returned error: %v", err)
	}

	img := &domain.Image{
		ID:        domain.NewID(),
		OwnerType: "person",
		OwnerID:   "test-owner",
		ImageType: domain.ImageTypePoster,
		URL:       "person/te/test-owner-never-put.jpg",
	}
	if err := imageRepo.Create(ctx, img); err != nil {
		t.Fatalf("imageRepo.Create returned error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /media/images/{id}", newImageHandler(imageRepo, imageStore, nil))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/media/images/" + img.ID) //nolint:noctx,gosec // test-only, URL is a fixed httptest.Server address
	if err != nil {
		t.Fatalf("http.Get returned error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
