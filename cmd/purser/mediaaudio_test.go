package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"purser/internal/domain"
	"purser/internal/ports"
	"testing"
)

// fakeMediaFileLister is a hand-rolled fake against the narrow
// mediaFileLister interface — per docs/adr/0003-go-testing-standards.md,
// mocks are only for ports, never a concrete struct, and a CMD-layer
// handler test fakes its dependencies rather than standing up a real
// adapter.
type fakeMediaFileLister struct {
	files []*domain.MediaFile
	err   error
}

func (f *fakeMediaFileLister) List(_ context.Context, _ string, _ int, _ string) ([]*domain.MediaFile, string, error) {
	if f.err != nil {
		return nil, "", f.err
	}
	return f.files, "", nil
}

func newAudioTestMux(lister mediaFileLister) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /media/audio/{itemId}", newAudioHandler(lister, nil))
	return mux
}

// writeTempAudioFile writes data to a temp file with the given extension so
// http.ServeContent's own name-based sniffing has something to work with.
func writeTempAudioFile(t *testing.T, ext string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "track"+ext)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("os.WriteFile returned error: %v", err)
	}
	return path
}

func TestNewAudioHandler_ServesBytes(t *testing.T) {
	itemID := "0198c1b0-0000-7000-8000-000000000010"
	want := []byte("purser-test-audio-bytes")
	path := writeTempAudioFile(t, ".mp3", want)
	lister := &fakeMediaFileLister{files: []*domain.MediaFile{{ID: domain.NewID(), ItemID: itemID, Path: path}}}
	mux := newAudioTestMux(lister)

	req := httptest.NewRequest(http.MethodGet, "/media/audio/"+itemID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.Bytes(); string(got) != string(want) {
		t.Fatalf("body = %v, want %v", got, want)
	}
}

func TestNewAudioHandler_SupportsRangeRequests(t *testing.T) {
	itemID := "0198c1b0-0000-7000-8000-000000000011"
	data := []byte("0123456789")
	path := writeTempAudioFile(t, ".mp3", data)
	lister := &fakeMediaFileLister{files: []*domain.MediaFile{{ID: domain.NewID(), ItemID: itemID, Path: path}}}
	mux := newAudioTestMux(lister)

	req := httptest.NewRequest(http.MethodGet, "/media/audio/"+itemID, nil)
	req.Header.Set("Range", "bytes=2-4")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusPartialContent)
	}
	if got, want := rec.Body.String(), "234"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
	if got := rec.Header().Get("Content-Range"); got == "" {
		t.Fatal("Content-Range header is empty, want a byte range")
	}
	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("Accept-Ranges = %q, want %q", got, "bytes")
	}
}

func TestNewAudioHandler_NoMediaFileReturns404(t *testing.T) {
	itemID := "0198c1b0-0000-7000-8000-000000000012"
	lister := &fakeMediaFileLister{files: nil}
	mux := newAudioTestMux(lister)

	req := httptest.NewRequest(http.MethodGet, "/media/audio/"+itemID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestNewAudioHandler_RepoNotFoundReturns404(t *testing.T) {
	itemID := "0198c1b0-0000-7000-8000-000000000013"
	lister := &fakeMediaFileLister{err: ports.ErrNotFound}
	mux := newAudioTestMux(lister)

	req := httptest.NewRequest(http.MethodGet, "/media/audio/"+itemID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestNewAudioHandler_RepoErrorReturns500(t *testing.T) {
	itemID := "0198c1b0-0000-7000-8000-000000000014"
	lister := &fakeMediaFileLister{err: errors.New("boom")}
	mux := newAudioTestMux(lister)

	req := httptest.NewRequest(http.MethodGet, "/media/audio/"+itemID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestNewAudioHandler_MissingFileOnDiskReturns404(t *testing.T) {
	itemID := "0198c1b0-0000-7000-8000-000000000015"
	lister := &fakeMediaFileLister{files: []*domain.MediaFile{{ID: domain.NewID(), ItemID: itemID, Path: filepath.Join(t.TempDir(), "gone.mp3")}}}
	mux := newAudioTestMux(lister)

	req := httptest.NewRequest(http.MethodGet, "/media/audio/"+itemID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestNewAudioHandler_MalformedIDReturns404BeforeRepoLookup(t *testing.T) {
	lister := &fakeMediaFileLister{err: errors.New("mediaFiles.List must never be called for a malformed id")}
	mux := newAudioTestMux(lister)

	// Percent-encoded so http.ServeMux's own path-cleaning redirect doesn't
	// mask what this test proves: the handler's own uuid.Parse check
	// rejects a traversal-shaped id before it's passed to mediaFiles.List.
	req := httptest.NewRequest(http.MethodGet, "/media/audio/..%2F..%2Fetc%2Fpasswd", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
