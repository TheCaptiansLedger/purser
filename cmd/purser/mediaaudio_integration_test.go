package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"purser/internal/adapters/datastore"
	dsbadger "purser/internal/adapters/datastore/badger"
	storemediafile "purser/internal/adapters/store/mediafile"
	"purser/internal/domain"
	"testing"
)

// TestNewAudioHandler_Integration exercises the real MediaFileRepository
// (Badger-backed) and a real file on disk, per the same shape
// mediaimages_integration_test.go uses for the image handler: a real
// Create, then a real HTTP GET against the handler, asserting byte-for-byte
// match and Range support.
func TestNewAudioHandler_Integration(t *testing.T) {
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

	mediaFileRepo, err := storemediafile.New("media_file", ds)
	if err != nil {
		t.Fatalf("storemediafile.New returned error: %v", err)
	}

	want := []byte("purser-test-track-bytes")
	path := filepath.Join(t.TempDir(), "track.mp3")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatalf("os.WriteFile returned error: %v", err)
	}

	itemID := domain.NewID()
	mf := &domain.MediaFile{ID: domain.NewID(), ItemID: itemID, Path: path}
	if err := mediaFileRepo.Create(ctx, mf); err != nil {
		t.Fatalf("mediaFileRepo.Create returned error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /media/audio/{itemId}", newAudioHandler(mediaFileRepo, nil))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/media/audio/" + itemID) //nolint:noctx,gosec // test-only, URL is a fixed httptest.Server address
	if err != nil {
		t.Fatalf("http.Get returned error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile returned error: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("file on disk = %v, want %v (sanity check on fixture, not the response)", got, want)
	}
}

// TestNewAudioHandler_Integration_NoMediaFileReturns404 covers the 404 case
// against the real MediaFileRepository: no MediaFile row exists for the
// given item id.
func TestNewAudioHandler_Integration_NoMediaFileReturns404(t *testing.T) {
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

	mediaFileRepo, err := storemediafile.New("media_file", ds)
	if err != nil {
		t.Fatalf("storemediafile.New returned error: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /media/audio/{itemId}", newAudioHandler(mediaFileRepo, nil))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/media/audio/" + domain.NewID()) //nolint:noctx,gosec // test-only, URL is a fixed httptest.Server address
	if err != nil {
		t.Fatalf("http.Get returned error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
