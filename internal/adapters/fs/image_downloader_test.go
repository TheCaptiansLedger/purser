package fs

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestImageDownloader_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("\xff\xd8\xff"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	ext := NewImageDownloader(dir).Download(t.Context(), srv.URL, "entries", "abc123")
	if ext != ".jpg" {
		t.Fatalf("ext = %q, want .jpg", ext)
	}
	if _, err := os.Stat(ImagePath(dir, "entries", "abc123", ".jpg")); err != nil {
		t.Errorf("file missing: %v", err)
	}
}

func TestImageDownloader_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "err", http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	ext := NewImageDownloader(dir).Download(t.Context(), srv.URL, "entries", "abc123")
	if ext != "" {
		t.Errorf("ext = %q, want empty on HTTP 500", ext)
	}
	if _, err := os.Stat(ImagePath(dir, "entries", "abc123", ".jpg")); err == nil {
		t.Error("file created despite HTTP 500")
	}
}

func TestImageDownloader_NetworkError(t *testing.T) {
	ext := NewImageDownloader(t.TempDir()).Download(t.Context(), "http://127.0.0.1:0/img.jpg", "entries", "abc123")
	if ext != "" {
		t.Errorf("ext = %q, want empty on network error", ext)
	}
}

func TestImageDownloader_BadContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("data"))
	}))
	defer srv.Close()

	ext := NewImageDownloader(t.TempDir()).Download(t.Context(), srv.URL, "entries", "abc123")
	if ext != ".jpg" {
		t.Errorf("ext = %q, want .jpg for unknown content type", ext)
	}
}

func TestImageDownloader_PathTraversal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("\xff\xd8\xff"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	ext := NewImageDownloader(dir).Download(t.Context(), srv.URL, "entries", "../../etc/passwd")
	if ext != "" {
		t.Errorf("ext = %q, want empty for path traversal", ext)
	}
}

func TestImageDownloader_PartialDownload_NoFileLeft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("server does not support hijacking")
			return
		}
		conn, bufw, _ := hj.Hijack()
		defer conn.Close()
		// Advertise 1000 bytes but close after 2 — simulates a crash mid-copy.
		_, _ = bufw.WriteString("HTTP/1.1 200 OK\r\nContent-Type: image/jpeg\r\nContent-Length: 1000\r\n\r\n\xff\xd8")
		_ = bufw.Flush()
	}))
	defer srv.Close()

	dir := t.TempDir()
	ext := NewImageDownloader(dir).Download(t.Context(), srv.URL, "entries", "abc123")
	if ext != "" {
		t.Errorf("ext = %q, want empty on partial download", ext)
	}
	if _, err := os.Stat(ImagePath(dir, "entries", "abc123", ".jpg")); err == nil {
		t.Error("partial download left a file at the destination path")
	}
}

func TestExtFromContentType(t *testing.T) {
	cases := []struct{ ct, want string }{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/webp", ".webp"},
		{"image/gif", ".gif"},
		{"image/svg+xml", ".svg"},
		{"application/octet-stream", ".jpg"},
		{"", ".jpg"},
	}
	for _, tc := range cases {
		if got := extFromContentType(tc.ct); got != tc.want {
			t.Errorf("extFromContentType(%q) = %q, want %q", tc.ct, got, tc.want)
		}
	}
}
